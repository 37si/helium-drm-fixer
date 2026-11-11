package gmp

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"charm.land/log/v2"

	"hdf/internal/cdm"
)

const (
	versionsURL  = "https://product-details.mozilla.org/1.0/firefox_versions.json"
	updateURL    = "https://aus5.mozilla.org/update/3/GMP/%s/%s/%s/en-US/release/default/default/default/update.xml"
	addonID      = "gmp-widevinecdm"
	maxArchive   = 64 << 20
	maxExtracted = 256 << 20
	markerName   = ".hdf-gmp.json"
)

var (
	ErrNoHash      = errors.New("the update document has no SHA-512 digest")
	ErrBadURL      = errors.New("unexpected download host")
	ErrNotCRX      = errors.New("not a CRX package")
	ErrBadPackage  = errors.New("the package holds no usable WidevineCdm")
	ErrUnsupported = errors.New("no WidevineCdm build for this platform")
)

type Progress func(downloaded, total int64)

type Release struct {
	Version string
	URL     string
	Hash    string
	Size    int64
	Target  string
	Firefox string
}

type Client struct {
	http  *http.Client
	root  string
	log   *log.Logger
	agent string
}

func New(dir string, logger *log.Logger) (*Client, error) {
	if dir == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			return nil, fmt.Errorf("locate the user cache directory: %w", err)
		}
		dir = filepath.Join(base, "hdf")
	}

	root := filepath.Join(dir, "gmp")

	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create download cache: %w", err)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		return nil, fmt.Errorf("restrict download cache: %w", err)
	}

	info, err := os.Lstat(root)
	if err != nil {
		return nil, fmt.Errorf("inspect download cache: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("download cache %s is not a plain directory", root)
	}

	if logger == nil {
		logger = log.New(io.Discard)
	}

	return &Client{
		http:  &http.Client{Timeout: 10 * time.Minute},
		root:  root,
		log:   logger,
		agent: "hdf (WidevineCdm updater)",
	}, nil
}

func (c *Client) Dir() string { return c.root }

func (c *Client) Latest(ctx context.Context, firefox string) (Release, error) {
	if firefox == "" {
		version, err := c.firefoxVersion(ctx)
		if err != nil {
			return Release{}, err
		}
		firefox = version
	}

	target, err := Target()
	if err != nil {
		return Release{}, err
	}

	build := time.Now().UTC().Format("20060102150405")
	endpoint := fmt.Sprintf(updateURL, firefox, build, target)

	body, err := c.get(ctx, endpoint)
	if err != nil {
		return Release{}, fmt.Errorf("ask Mozilla for the current WidevineCdm: %w", err)
	}

	var document struct {
		Addons []struct {
			ID           string `xml:"id,attr"`
			URL          string `xml:"URL,attr"`
			HashFunction string `xml:"hashFunction,attr"`
			HashValue    string `xml:"hashValue,attr"`
			Size         int64  `xml:"size,attr"`
			Version      string `xml:"version,attr"`
		} `xml:"addons>addon"`
	}

	if err := xml.Unmarshal(body, &document); err != nil {
		return Release{}, fmt.Errorf("read the update document: %w", err)
	}

	for _, addon := range document.Addons {
		if addon.ID != addonID {
			continue
		}

		release := Release{
			Version: addon.Version,
			URL:     addon.URL,
			Hash:    strings.ToLower(strings.TrimSpace(addon.HashValue)),
			Size:    addon.Size,
			Target:  target,
			Firefox: firefox,
		}

		return release, release.validate()
	}

	return Release{}, fmt.Errorf("%w: no %s addon for %s", ErrBadPackage, addonID, target)
}

func (r Release) validate() error {
	if err := cdm.ValidateVersion(r.Version); err != nil {
		return err
	}

	if err := checkURL(r.URL); err != nil {
		return err
	}

	if len(r.Hash) != sha512.Size*2 {
		return fmt.Errorf("%w: %q", ErrNoHash, r.Hash)
	}
	if _, err := hex.DecodeString(r.Hash); err != nil {
		return fmt.Errorf("%w: %q", ErrNoHash, r.Hash)
	}

	if r.Size <= 0 || r.Size > maxArchive {
		return fmt.Errorf("%w: %d bytes", ErrBadPackage, r.Size)
	}

	return nil
}

var hosts = []string{
	"mozilla.org",
	"mozilla.net",
	"gvt1.com",
	"gvt2.com",
	"googleusercontent.com",
}

func checkURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBadURL, err)
	}

	if parsed.Scheme != "https" {
		return fmt.Errorf("%w: %s is not https", ErrBadURL, raw)
	}

	host := strings.ToLower(parsed.Hostname())
	for _, allowed := range hosts {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return nil
		}
	}

	return fmt.Errorf("%w: %s", ErrBadURL, host)
}

func (c *Client) firefoxVersion(ctx context.Context) (string, error) {
	body, err := c.get(ctx, versionsURL)
	if err != nil {
		return "", fmt.Errorf("look up the newest Firefox version: %w", err)
	}

	var versions struct {
		Release string `json:"LATEST_FIREFOX_VERSION"`
	}
	if err := json.Unmarshal(body, &versions); err != nil {
		return "", fmt.Errorf("read the Firefox version list: %w", err)
	}

	if versions.Release == "" {
		return "", errors.New("Mozilla published no Firefox release version")
	}

	return versions.Release, nil
}

func (c *Client) get(ctx context.Context, endpoint string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", c.agent)

	response, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s", response.Status)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxArchive))
	if err != nil {
		return nil, err
	}

	return body, nil
}

func (c *Client) Fetch(ctx context.Context, release Release, progress Progress) (string, error) {
	if err := release.validate(); err != nil {
		return "", err
	}

	dest := filepath.Join(c.root, release.Version)
	if !contained(c.root, dest) {
		return "", fmt.Errorf("%w: %s", cdm.ErrUnsafeVersion, release.Version)
	}

	if c.cached(dest, release) {
		c.log.Info("using the cached WidevineCdm", "version", release.Version)
		return dest, nil
	}

	archive, err := c.download(ctx, release, progress)
	if err != nil {
		return "", err
	}
	defer os.Remove(archive)

	payload, err := readArchive(archive, release.Size)
	if err != nil {
		return "", err
	}

	dir, err := c.unpack(payload, release)
	if err != nil {
		return "", err
	}

	if err := os.RemoveAll(dest); err != nil {
		return "", err
	}
	if err := os.Rename(dir, dest); err != nil {
		os.RemoveAll(dir)
		return "", err
	}

	if err := writeMarker(dest, release); err != nil {
		c.log.Warn("could not record the fetched build", "err", err)
	}

	return dest, nil
}

func (c *Client) download(ctx context.Context, release Release, progress Progress) (string, error) {
	c.log.Info("downloading WidevineCdm", "version", release.Version, "bytes", release.Size)

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, release.URL, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", c.agent)

	response, err := c.http.Do(request)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download: unexpected status %s", response.Status)
	}

	file, err := os.CreateTemp(c.root, "widevine-*.crx")
	if err != nil {
		return "", err
	}

	name := file.Name()

	written, copyErr := copyWithProgress(file, io.LimitReader(response.Body, maxArchive), release.Size, progress)
	closeErr := file.Close()

	switch {
	case copyErr != nil:
		os.Remove(name)
		return "", copyErr
	case closeErr != nil:
		os.Remove(name)
		return "", closeErr
	case written != release.Size:
		os.Remove(name)
		return "", fmt.Errorf("download is %d bytes, the update document says %d", written, release.Size)
	}

	sum, err := digest(name)
	if err != nil {
		os.Remove(name)
		return "", err
	}

	if sum != release.Hash {
		os.Remove(name)
		return "", fmt.Errorf("%w: got %s, wanted %s", ErrBadPackage, sum, release.Hash)
	}

	return name, nil
}

func readArchive(path string, size int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	raw, err := io.ReadAll(io.LimitReader(file, min(size+1<<20, maxArchive)))
	if err != nil {
		return nil, err
	}

	return crxPayload(raw)
}

func crxPayload(raw []byte) ([]byte, error) {
	if len(raw) < 16 || string(raw[:4]) != "Cr24" {
		return nil, ErrNotCRX
	}

	version := binary.LittleEndian.Uint32(raw[4:8])

	var start uint64
	switch version {
	case 3:
		start = 12 + uint64(binary.LittleEndian.Uint32(raw[8:12]))
	case 2:
		public := uint64(binary.LittleEndian.Uint32(raw[8:12]))
		signature := uint64(binary.LittleEndian.Uint32(raw[12:16]))
		start = 16 + public + signature
	default:
		return nil, fmt.Errorf("%w: version %d", ErrNotCRX, version)
	}

	if start >= uint64(len(raw)) {
		return nil, fmt.Errorf("%w: header runs past the end of the file", ErrNotCRX)
	}

	return raw[start:], nil
}

func (c *Client) unpack(payload []byte, release Release) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrBadPackage, err)
	}

	staging, err := os.MkdirTemp(c.root, "extract-")
	if err != nil {
		return "", err
	}

	cleanup := func() { os.RemoveAll(staging) }

	var total int64
	for _, entry := range reader.File {
		target, err := safeJoin(staging, entry.Name)
		if err != nil {
			cleanup()
			return "", err
		}

		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				cleanup()
				return "", err
			}
			continue
		}

		total += int64(entry.UncompressedSize64)
		if total > maxExtracted {
			cleanup()
			return "", fmt.Errorf("%w: the archive unpacks to more than %d bytes", ErrBadPackage, maxExtracted)
		}

		if err := extractFile(entry, target); err != nil {
			cleanup()
			return "", err
		}
	}

	dir, err := findCDM(staging)
	if err != nil {
		cleanup()
		return "", err
	}

	if err := checkPlatform(dir); err != nil {
		cleanup()
		return "", err
	}

	if dir == staging {
		return staging, nil
	}

	inner, err := os.MkdirTemp(c.root, "bundle-")
	if err != nil {
		cleanup()
		return "", err
	}

	if err := os.Rename(dir, filepath.Join(inner, "WidevineCdm")); err != nil {
		os.RemoveAll(inner)
		cleanup()
		return "", err
	}

	cleanup()

	c.log.Info("unpacked WidevineCdm", "version", release.Version)

	return filepath.Join(inner, "WidevineCdm"), nil
}

func safeJoin(root, name string) (string, error) {
	raw := filepath.FromSlash(name)
	separator := string(filepath.Separator)

	if strings.HasPrefix(raw, separator) || strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, `\`) {
		return "", fmt.Errorf("%w: %q escapes the archive", ErrBadPackage, name)
	}
	if filepath.VolumeName(raw) != "" {
		return "", fmt.Errorf("%w: %q escapes the archive", ErrBadPackage, name)
	}

	clean := filepath.Clean(raw)
	if clean == "." {
		return root, nil
	}
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+separator) {
		return "", fmt.Errorf("%w: %q escapes the archive", ErrBadPackage, name)
	}

	target := filepath.Join(root, clean)
	if !contained(root, target) && target != root {
		return "", fmt.Errorf("%w: %q escapes the archive", ErrBadPackage, name)
	}

	return target, nil
}

func extractFile(entry *zip.File, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	source, err := entry.Open()
	if err != nil {
		return err
	}
	defer source.Close()

	mode := os.FileMode(0o644)
	if entry.Mode()&0o111 != 0 {
		mode = 0o755
	}

	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}

	if _, err := io.Copy(file, io.LimitReader(source, maxExtracted)); err != nil {
		file.Close()
		return err
	}

	return file.Close()
}

func findCDM(root string) (string, error) {
	if cdm.Valid(root) {
		return root, nil
	}

	var found string

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || found != "" {
			return err
		}

		if entry.IsDir() && cdm.Valid(path) {
			found = path
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	if found == "" {
		return "", fmt.Errorf("%w: no manifest.json", ErrBadPackage)
	}

	return found, nil
}

func checkPlatform(dir string) error {
	want := PlatformDir()

	entries, err := os.ReadDir(filepath.Join(dir, "_platform_specific"))
	if err != nil {
		return fmt.Errorf("%w: the package has no %s", ErrBadPackage, want)
	}

	for _, entry := range entries {
		if entry.IsDir() && entry.Name() == want {
			return nil
		}
	}

	return fmt.Errorf("%w: the package has no %s", ErrBadPackage, want)
}

func Target() (string, error) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "windows/amd64":
		return "WINNT_x86_64-msvc-x64", nil
	case "windows/arm64":
		return "WINNT_aarch64-msvc-aarch64", nil
	case "linux/amd64":
		return "Linux_x86_64-gcc3", nil
	case "linux/arm64":
		return "Linux_aarch64-gcc3", nil
	case "darwin/amd64":
		return "Darwin_x86_64-gcc3-u-i386-x86_64", nil
	case "darwin/arm64":
		return "Darwin_aarch64-gcc3", nil
	}

	return "", fmt.Errorf("%w: %s/%s", ErrUnsupported, runtime.GOOS, runtime.GOARCH)
}

func PlatformDir() string {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "windows/arm64":
		return "win_arm64"
	case "windows/amd64":
		return "win_x64"
	case "linux/arm64":
		return "linux_arm64"
	case "linux/amd64":
		return "linux_x64"
	case "darwin/arm64":
		return "mac_arm64"
	}

	return "mac_x64"
}

type marker struct {
	Version string    `json:"version"`
	Hash    string    `json:"sha512"`
	Target  string    `json:"target"`
	Firefox string    `json:"firefox"`
	Fetched time.Time `json:"fetched_at"`
}

func writeMarker(dir string, release Release) error {
	payload, err := json.MarshalIndent(marker{
		Version: release.Version,
		Hash:    release.Hash,
		Target:  release.Target,
		Firefox: release.Firefox,
		Fetched: time.Now(),
	}, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, markerName), append(payload, '\n'), 0o644)
}

func (c *Client) cached(dir string, release Release) bool {
	if !cdm.Valid(dir) {
		return false
	}

	raw, err := os.ReadFile(filepath.Join(dir, markerName))
	if err != nil {
		return false
	}

	var record marker
	if err := json.Unmarshal(raw, &record); err != nil {
		return false
	}

	if record.Version != release.Version || record.Hash != release.Hash || record.Target != release.Target {
		return false
	}

	version, err := cdm.Version(dir)
	if err != nil || version != release.Version {
		return false
	}

	return true
}

func digest(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha512.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func copyWithProgress(dst io.Writer, src io.Reader, total int64, progress Progress) (int64, error) {
	buffer := make([]byte, 128*1024)
	var written int64

	for {
		read, err := src.Read(buffer)
		if read > 0 {
			if _, writeErr := dst.Write(buffer[:read]); writeErr != nil {
				return written, writeErr
			}

			written += int64(read)
			if progress != nil {
				progress(written, total)
			}
		}

		if errors.Is(err, io.EOF) {
			return written, nil
		}
		if err != nil {
			return written, err
		}
	}
}

func contained(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil || filepath.IsAbs(rel) || rel == "." {
		return false
	}

	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
