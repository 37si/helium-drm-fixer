package update

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"hdf/internal/pathutil"
)

var ErrNotFound = errors.New("no release found")

var ErrChecksum = errors.New("checksum mismatch")

type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

type Release struct {
	Tag     string  `json:"tag_name"`
	PageURL string  `json:"html_url"`
	Assets  []Asset `json:"assets"`
}

func (r Release) Version() string { return strings.TrimPrefix(r.Tag, "v") }

func (r Release) Binary() (Asset, bool) {
	return r.Asset(runtime.GOOS, runtime.GOARCH)
}

func (r Release) Asset(goos, goarch string) (Asset, bool) {
	want := BinaryName(r.Version(), goos, goarch)

	for _, asset := range r.Assets {
		if asset.Name == want {
			return asset, true
		}
	}

	return Asset{}, false
}

func (r Release) Checksums() (Asset, bool) {
	for _, asset := range r.Assets {
		if asset.Name == "sha256.txt" {
			return asset, true
		}
	}

	return Asset{}, false
}

func BinaryName(version, goos, goarch string) string {
	name := fmt.Sprintf("hdf_%s_%s_%s", version, goos, goarch)
	if goos == "windows" {
		name += ".exe"
	}

	return name
}

func Latest(ctx context.Context, repo string) (Release, error) {
	if repo == "" {
		return Release{}, errors.New("no repository configured; pass --repo owner/name")
	}

	url := "https://api.github.com/repos/" + repo + "/releases/latest"

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Release{}, err
	}

	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "hdf")

	client := &http.Client{Timeout: 15 * time.Second}

	response, err := client.Do(request)
	if err != nil {
		return Release{}, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		return Release{}, fmt.Errorf("%w: %s", ErrNotFound, repo)
	}
	if response.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("github: %s", response.Status)
	}

	var release Release
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&release); err != nil {
		return Release{}, err
	}

	return release, nil
}

func Newer(candidate, current string) bool {
	candidate = strings.TrimPrefix(candidate, "v")
	current = strings.TrimPrefix(current, "v")

	if candidate == "" {
		return false
	}

	return pathutil.CompareVersions(candidate, current) > 0
}

func Download(ctx context.Context, asset Asset, dst string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return err
	}

	request.Header.Set("User-Agent", "hdf")

	client := &http.Client{Timeout: 5 * time.Minute}

	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", asset.Name, response.Status)
	}

	file, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}

	if _, err := io.Copy(file, response.Body); err != nil {
		file.Close()
		return err
	}

	return file.Close()
}

func Sums(ctx context.Context, asset Asset, name string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return "", err
	}

	request.Header.Set("User-Agent", "hdf")

	client := &http.Client{Timeout: 30 * time.Second}

	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download checksums: %s", response.Status)
	}

	scanner := bufio.NewScanner(io.LimitReader(response.Body, 1<<20))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			continue
		}
		if strings.TrimPrefix(fields[1], "*") == name {
			return strings.ToLower(fields[0]), nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read checksums: %w", err)
	}

	return "", fmt.Errorf("%w: %s is not listed", ErrNotFound, name)
}

func Digest(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
