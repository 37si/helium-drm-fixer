package cdm

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeCDM(t *testing.T, dir, version string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(dir, "_platform_specific", "linux_x64"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"version":"`+version+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "_platform_specific", "linux_x64", "libwidevinecdm.so"), []byte("so"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestValid(t *testing.T) {
	t.Run("manifest", func(t *testing.T) {
		dir := t.TempDir()
		writeCDM(t, dir, "4.10.2891.0")
		if !Valid(dir) {
			t.Fatal("expected the CDM to be valid")
		}
	})

	t.Run("platform specific only", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "_platform_specific"), 0o755); err != nil {
			t.Fatal(err)
		}
		if !Valid(dir) {
			t.Fatal("expected _platform_specific to be enough")
		}
	})

	t.Run("loose library", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "widevinecdm.dll"), []byte("dll"), 0o644); err != nil {
			t.Fatal(err)
		}
		if !Valid(dir) {
			t.Fatal("expected a widevinecdm library to be enough")
		}
	})

	t.Run("empty", func(t *testing.T) {
		if Valid(t.TempDir()) {
			t.Fatal("an empty directory is not a CDM")
		}
	})

	t.Run("missing", func(t *testing.T) {
		if Valid(filepath.Join(t.TempDir(), "nope")) {
			t.Fatal("a missing directory is not a CDM")
		}
	})

	t.Run("file", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "manifest.json")
		if err := os.WriteFile(file, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		if Valid(file) {
			t.Fatal("a file is not a CDM directory")
		}
	})
}

func TestVersion(t *testing.T) {
	t.Run("direct manifest", func(t *testing.T) {
		dir := t.TempDir()
		writeCDM(t, dir, "4.10.2891.0")

		version, err := Version(dir)
		if err != nil {
			t.Fatal(err)
		}
		if version != "4.10.2891.0" {
			t.Fatalf("Version = %q", version)
		}
	})

	t.Run("nested manifest", func(t *testing.T) {
		dir := t.TempDir()
		nested := filepath.Join(dir, "4.10.2891.0")
		writeCDM(t, nested, "4.10.2891.0")

		version, err := Version(dir)
		if err != nil {
			t.Fatal(err)
		}
		if version != "4.10.2891.0" {
			t.Fatalf("Version = %q", version)
		}
	})

	t.Run("no manifest", func(t *testing.T) {
		if _, err := Version(t.TempDir()); !errors.Is(err, ErrInvalid) {
			t.Fatalf("expected ErrInvalid, got %v", err)
		}
	})
}

func TestCopy(t *testing.T) {
	src := t.TempDir()
	writeCDM(t, src, "4.10.2891.0")

	nested := filepath.Join(src, "nested", "deep")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "file.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "target")
	if err := Copy(src, dst); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(dst, "nested", "deep", "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "hello" {
		t.Fatalf("copied content = %q", raw)
	}

	if !Valid(dst) {
		t.Fatal("copied directory should still be a valid CDM")
	}
}

func TestCopyReplacesDestination(t *testing.T) {
	src := t.TempDir()
	writeCDM(t, src, "4.10.2891.0")

	dst := t.TempDir()
	stale := filepath.Join(dst, "stale.txt")
	if err := os.WriteFile(stale, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Copy(src, dst); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(stale); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("destination should have been cleared")
	}
}

func TestCopyRejectsInvalidSource(t *testing.T) {
	if err := Copy(t.TempDir(), filepath.Join(t.TempDir(), "dst")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid, got %v", err)
	}
}

func TestValidateVersion(t *testing.T) {
	for _, version := range []string{"4.10.2934.0", "4.10.2891.0", "1.2.3", "4.10.2934.0-beta", "v1"} {
		if err := ValidateVersion(version); err != nil {
			t.Errorf("ValidateVersion(%q) = %v, want nil", version, err)
		}
	}

	for _, version := range []string{
		"", ".", "..", "../x", `..\x`, "/abs", "a/b", "4.10/../../x",
		".hidden", "4.10.2934.0:stream", strings.Repeat("9", maxVersionLen+1),
	} {
		if err := ValidateVersion(version); !errors.Is(err, ErrUnsafeVersion) {
			t.Errorf("ValidateVersion(%q) = %v, want ErrUnsafeVersion", version, err)
		}
	}
}

func TestVersionRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	writeCDM(t, dir, "../../../../tmp/PWNED")

	if _, err := Version(dir); !errors.Is(err, ErrUnsafeVersion) {
		t.Fatalf("Version() = %v, want ErrUnsafeVersion", err)
	}
}

func TestCopyRejectsOverlap(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "WidevineCdm")
	writeCDM(t, src, "4.10.2934.0")

	for _, dst := range []string{src, filepath.Join(src, "nested"), dir} {
		if err := Copy(src, dst); !errors.Is(err, ErrOverlap) {
			t.Errorf("Copy(%s) = %v, want ErrOverlap", dst, err)
		}
	}

	if !Valid(src) {
		t.Fatal("the source was damaged by the refused copies")
	}
}

func TestCopyRejectsEscapingSymlink(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "WidevineCdm")
	writeCDM(t, src, "4.10.2934.0")

	if err := os.Symlink("/etc/passwd", filepath.Join(src, "escape.so")); err != nil {
		t.Skipf("symlinks are not available: %v", err)
	}

	if err := Copy(src, filepath.Join(dir, "out")); err == nil {
		t.Fatal("expected a symlink pointing outside the bundle to be refused")
	}
}

func TestCopyFollowsSymlinkedSource(t *testing.T) {
	real := filepath.Join(t.TempDir(), "WidevineCdm")
	writeCDM(t, real, "4.10.2934.0")

	link := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks are not available: %v", err)
	}

	dst := filepath.Join(t.TempDir(), "out")
	if err := Copy(link, dst); err != nil {
		t.Fatalf("Copy() = %v, want nil", err)
	}

	if !Valid(dst) {
		t.Fatal("the symlinked source copied nothing")
	}
	if got, err := Version(dst); err != nil || got != "4.10.2934.0" {
		t.Fatalf("version = %q, %v", got, err)
	}
}

func TestCopyLeavesDestinationWhenTheSourceFails(t *testing.T) {
	src := filepath.Join(t.TempDir(), "WidevineCdm")
	writeCDM(t, src, "4.10.2934.0")

	if err := os.Symlink("/etc/passwd", filepath.Join(src, "escape.so")); err != nil {
		t.Skipf("symlinks are not available: %v", err)
	}

	dst := filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(dst, "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Copy(src, dst); err == nil {
		t.Fatal("expected the escaping symlink to be refused")
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("a failed copy removed the destination: %v", err)
	}
}

func TestCopyKeepsInternalSymlink(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "WidevineCdm")
	writeCDM(t, src, "4.10.2934.0")

	link := filepath.Join(src, "_platform_specific", "linux_x64", "libwidevinecdm.so.1")
	if err := os.Symlink("libwidevinecdm.so", link); err != nil {
		t.Skipf("symlinks are not available: %v", err)
	}

	dst := filepath.Join(dir, "out")
	if err := Copy(src, dst); err != nil {
		t.Fatalf("Copy() = %v, want nil", err)
	}

	target, err := os.Readlink(filepath.Join(dst, "_platform_specific", "linux_x64", "libwidevinecdm.so.1"))
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if target != "libwidevinecdm.so" {
		t.Fatalf("link points at %q", target)
	}
}
