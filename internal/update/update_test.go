package update

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestBinaryName(t *testing.T) {
	cases := []struct {
		goos string
		want string
	}{
		{"linux", "hdf_3.0.0_linux_amd64"},
		{"darwin", "hdf_3.0.0_darwin_amd64"},
		{"windows", "hdf_3.0.0_windows_amd64.exe"},
	}

	for _, c := range cases {
		if got := BinaryName("3.0.0", c.goos, "amd64"); got != c.want {
			t.Fatalf("BinaryName(%s) = %q, want %q", c.goos, got, c.want)
		}
	}
}

func TestNewer(t *testing.T) {
	cases := []struct {
		candidate string
		current   string
		want      bool
	}{
		{"3.0.1", "3.0.0", true},
		{"v3.1.0", "3.0.9", true},
		{"3.0.0", "3.0.0", false},
		{"2.9.9", "3.0.0", false},
		{"", "3.0.0", false},
	}

	for _, c := range cases {
		if got := Newer(c.candidate, c.current); got != c.want {
			t.Fatalf("Newer(%q, %q) = %v", c.candidate, c.current, got)
		}
	}
}

func TestReleaseAssets(t *testing.T) {
	release := Release{
		Tag: "v3.0.1",
		Assets: []Asset{
			{Name: "hdf_3.0.1_linux_amd64", URL: "https://example.test/linux"},
			{Name: "hdf_3.0.1_windows_amd64.exe", URL: "https://example.test/windows"},
			{Name: "sha256.txt", URL: "https://example.test/sums"},
		},
	}

	if release.Version() != "3.0.1" {
		t.Fatalf("version = %q", release.Version())
	}

	asset, ok := release.Asset("linux", "amd64")
	if !ok || asset.URL != "https://example.test/linux" {
		t.Fatalf("linux asset = %+v", asset)
	}

	asset, ok = release.Asset("windows", "amd64")
	if !ok || asset.URL != "https://example.test/windows" {
		t.Fatalf("windows asset = %+v", asset)
	}

	if _, ok := release.Asset("linux", "arm64"); ok {
		t.Fatal("found an asset that was never published")
	}

	if sums, ok := release.Checksums(); !ok || sums.Name != "sha256.txt" {
		t.Fatalf("checksums = %+v", sums)
	}
}

func TestDigest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "binary")
	payload := []byte("hdf")

	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatal(err)
	}

	sum := sha256.Sum256(payload)
	want := hex.EncodeToString(sum[:])

	got, err := Digest(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("digest = %q, want %q", got, want)
	}
}
