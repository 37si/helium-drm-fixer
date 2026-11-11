package gmp

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"hdf/internal/cdm"
)

func crx(payload []byte) []byte {
	header := []byte{1, 2, 3, 4, 5}

	out := make([]byte, 12, 12+len(header)+len(payload))
	copy(out, "Cr24")
	binary.LittleEndian.PutUint32(out[4:], 3)
	binary.LittleEndian.PutUint32(out[8:], uint32(len(header)))

	return append(append(out, header...), payload...)
}

func zipped(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)

	for name, body := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	return buffer.Bytes()
}

func TestCRX3PayloadIsTheZip(t *testing.T) {
	payload := []byte("PK-payload")

	got, err := crxPayload(crx(payload))
	if err != nil {
		t.Fatalf("crxPayload: %v", err)
	}

	if string(got) != string(payload) {
		t.Fatalf("payload = %q, want %q", got, payload)
	}
}

func TestCRX2HeaderIsSkipped(t *testing.T) {
	payload := []byte("zip-bytes")

	out := make([]byte, 16, 16+3+4+len(payload))
	copy(out, "Cr24")
	binary.LittleEndian.PutUint32(out[4:], 2)
	binary.LittleEndian.PutUint32(out[8:], 3)
	binary.LittleEndian.PutUint32(out[12:], 4)

	out = append(out, 0, 0, 0, 0, 0, 0, 0)
	out = append(out, payload...)

	got, err := crxPayload(out)
	if err != nil {
		t.Fatalf("crxPayload: %v", err)
	}

	if string(got) != string(payload) {
		t.Fatalf("payload = %q, want %q", got, payload)
	}
}

func TestCRXRejectsBadHeaders(t *testing.T) {
	cases := map[string][]byte{
		"empty":     nil,
		"short":     []byte("Cr24\x03\x00"),
		"magic":     append([]byte("Nope"), make([]byte, 16)...),
		"version":   append([]byte("Cr24\x09\x00\x00\x00"), make([]byte, 16)...),
		"overrun":   append([]byte("Cr24\x03\x00\x00\x00\xff\xff\x00\x00"), make([]byte, 8)...),
		"truncated": append([]byte("Cr24\x03\x00\x00\x00"), make([]byte, 4)...),
	}

	for name, raw := range cases {
		if _, err := crxPayload(raw); err == nil {
			t.Fatalf("%s: expected an error", name)
		}
	}
}

func TestSafeJoinRefusesEscapes(t *testing.T) {
	root := t.TempDir()

	for _, name := range []string{"../evil", "a/../../evil", "/etc/passwd", `..\evil`} {
		if _, err := safeJoin(root, name); err == nil {
			t.Fatalf("%q was accepted", name)
		}
	}

	for _, name := range []string{"manifest.json", "_platform_specific/win_x64/widevinecdm.dll", "./LICENSE.txt"} {
		if _, err := safeJoin(root, name); err != nil {
			t.Fatalf("%q was refused: %v", name, err)
		}
	}
}

func TestUnpackExtractsAndFindsTheBundle(t *testing.T) {
	client := &Client{root: t.TempDir()}

	platform := PlatformDir()
	payload := zipped(t, map[string]string{
		"manifest.json": `{"version":"4.10.3112.0"}`,
		"LICENSE.txt":   "license",
		filepath.ToSlash(filepath.Join("_platform_specific", platform, "widevinecdm.bin")): "binary",
	})

	dir, err := client.unpack(payload, Release{Version: "4.10.3112.0"})
	if err != nil {
		t.Fatalf("unpack: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err != nil {
		t.Fatalf("manifest is missing: %v", err)
	}

	if got, err := cdm.Version(dir); err != nil || got != "4.10.3112.0" {
		t.Fatalf("version = %q, %v", got, err)
	}
}

func TestUnpackRefusesTraversal(t *testing.T) {
	client := &Client{root: t.TempDir()}

	payload := zipped(t, map[string]string{"../../escape.txt": "nope"})

	if _, err := client.unpack(payload, Release{Version: "4.10.3112.0"}); err == nil {
		t.Fatal("a zip entry escaping the directory was accepted")
	}
}

func TestUnpackRefusesAnotherPlatform(t *testing.T) {
	client := &Client{root: t.TempDir()}

	other := "win_x64"
	if PlatformDir() == other {
		other = "linux_x64"
	}

	payload := zipped(t, map[string]string{
		"manifest.json": `{"version":"4.10.3112.0"}`,
		filepath.ToSlash(filepath.Join("_platform_specific", other, "widevinecdm.bin")): "binary",
	})

	if _, err := client.unpack(payload, Release{Version: "4.10.3112.0"}); err == nil {
		t.Fatal("a package for another platform was accepted")
	}
}

func TestUnpackRefusesPackageWithoutPlatform(t *testing.T) {
	client := &Client{root: t.TempDir()}

	payload := zipped(t, map[string]string{
		"manifest.json": `{"version":"4.10.3112.0"}`,
	})

	if _, err := client.unpack(payload, Release{Version: "4.10.3112.0"}); err == nil {
		t.Fatal("a package without a platform directory was accepted")
	}
}

func TestContained(t *testing.T) {
	root := t.TempDir()

	if !contained(root, filepath.Join(root, "a", "b")) {
		t.Fatal("a nested path should be contained")
	}

	for _, path := range []string{root, filepath.Dir(root), filepath.Join(root, "..", "x")} {
		if contained(root, path) {
			t.Fatalf("%q should not be contained", path)
		}
	}
}

func TestReleaseValidation(t *testing.T) {
	digest := strings.Repeat("ab", 64)

	valid := Release{
		Version: "4.10.3112.0",
		URL:     "https://aus5.mozilla.org/update/3/GMP/x.crx3",
		Hash:    digest,
		Size:    1024,
	}

	if err := valid.validate(); err != nil {
		t.Fatalf("valid release refused: %v", err)
	}

	cases := map[string]func(*Release){
		"no hash":        func(r *Release) { r.Hash = "" },
		"short hash":     func(r *Release) { r.Hash = "abcd" },
		"not hex":        func(r *Release) { r.Hash = strings.Repeat("zz", 64) },
		"no version":     func(r *Release) { r.Version = "" },
		"traversal":      func(r *Release) { r.Version = "../../etc" },
		"http":           func(r *Release) { r.URL = "http://aus5.mozilla.org/x" },
		"foreign host":   func(r *Release) { r.URL = "https://example.com/x.crx3" },
		"no size":        func(r *Release) { r.Size = 0 },
		"absurd size":    func(r *Release) { r.Size = 1 << 40 },
		"lookalike host": func(r *Release) { r.URL = "https://mozilla.org.evil.example/x" },
	}

	for name, breakIt := range cases {
		broken := valid
		breakIt(&broken)

		if err := broken.validate(); err == nil {
			t.Fatalf("%s: expected an error", name)
		}
	}
}

func TestCheckURLAllowsMozillaHosts(t *testing.T) {
	good := []string{
		"https://aus5.mozilla.org/update/3/GMP/x",
		"https://mozilla.org/x",
		"https://redirector.gvt1.com/edgedl/x.crx3",
		"https://cdn.mozilla.net/x",
	}

	for _, raw := range good {
		if err := checkURL(raw); err != nil {
			t.Fatalf("%s refused: %v", raw, err)
		}
	}
}

func TestTargetMatchesThePlatform(t *testing.T) {
	target, err := Target()

	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "windows/amd64", "linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64", "windows/arm64":
		if err != nil {
			t.Fatalf("Target: %v", err)
		}
		if target == "" {
			t.Fatal("Target is empty")
		}
	default:
		if err == nil {
			t.Fatalf("Target accepted an unsupported platform: %q", target)
		}
	}

	if PlatformDir() == "" {
		t.Fatal("PlatformDir is empty")
	}
}

func TestDigestIsSha512(t *testing.T) {
	path := filepath.Join(t.TempDir(), "payload")
	if err := os.WriteFile(path, []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := digest(path)
	if err != nil {
		t.Fatal(err)
	}

	const want = "ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a" +
		"2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f"

	if got != want {
		t.Fatalf("digest = %s", got)
	}

	if _, err := hex.DecodeString(got); err != nil {
		t.Fatalf("digest is not hex: %v", err)
	}
}
