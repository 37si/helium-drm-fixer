//go:build linux

package helium

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func testHome(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	return home
}

func TestFindOverrideVersioned(t *testing.T) {
	root := t.TempDir()
	version := filepath.Join(root, "131.0.6778.86")
	if err := os.Mkdir(version, 0o755); err != nil {
		t.Fatal(err)
	}

	install, err := Find(root)
	if err != nil {
		t.Fatal(err)
	}
	if install.Dir != version {
		t.Fatalf("Dir = %q, want %q", install.Dir, version)
	}
	if install.Root != root {
		t.Fatalf("Root = %q", install.Root)
	}
}

func TestFindOverrideFlat(t *testing.T) {
	root := t.TempDir()

	install, err := Find(root)
	if err != nil {
		t.Fatal(err)
	}
	if install.Dir != root {
		t.Fatalf("Dir = %q, want %q", install.Dir, root)
	}
}

func TestFindOverrideMissing(t *testing.T) {
	if _, err := Find(filepath.Join(t.TempDir(), "missing")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestTarget(t *testing.T) {
	home := testHome(t)

	userData := filepath.Join(home, ".config", "net.imput.helium")
	if err := os.MkdirAll(userData, 0o755); err != nil {
		t.Fatal(err)
	}

	target, hint, err := Target(Install{}, "4.10.2891.0")
	if err != nil {
		t.Fatal(err)
	}

	wantTarget := filepath.Join(userData, "WidevineCdm", "4.10.2891.0")
	if target != wantTarget {
		t.Fatalf("target = %q, want %q", target, wantTarget)
	}

	wantHint := filepath.Join(userData, "WidevineCdm", HintFile)
	if hint != wantHint {
		t.Fatalf("hint = %q, want %q", hint, wantHint)
	}
}

func TestTargetWithoutUserData(t *testing.T) {
	testHome(t)

	if _, _, err := Target(Install{}, "4.10.2891.0"); !errors.Is(err, ErrNoUserData) {
		t.Fatalf("expected ErrNoUserData, got %v", err)
	}
}

func TestWriteHint(t *testing.T) {
	hint := filepath.Join(t.TempDir(), "nested", HintFile)

	if err := WriteHint(hint, "/some/where/WidevineCdm/4.10.2891.0"); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(hint)
	if err != nil {
		t.Fatal(err)
	}

	var payload map[string]string
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}

	if payload["Path"] != "/some/where/WidevineCdm/4.10.2891.0" {
		t.Fatalf("payload = %v", payload)
	}
}

func writeInstalledCDM(t *testing.T, dir, version string) string {
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

	return dir
}

func TestInstalledListsNewestFirst(t *testing.T) {
	home := testHome(t)

	root := filepath.Join(home, ".config", "net.imput.helium", "WidevineCdm")
	writeInstalledCDM(t, filepath.Join(root, "4.10.2900.0"), "4.10.2900.0")
	writeInstalledCDM(t, filepath.Join(root, "4.10.2934.0"), "4.10.2934.0")

	found := Installed(Install{})
	if len(found) != 2 {
		t.Fatalf("found %d CDMs", len(found))
	}
	if found[0].Version != "4.10.2934.0" {
		t.Fatalf("first = %q", found[0].Version)
	}
}

func TestInstalledFollowsHint(t *testing.T) {
	home := testHome(t)

	root := filepath.Join(home, ".config", "net.imput.helium", "WidevineCdm")
	older := writeInstalledCDM(t, filepath.Join(root, "4.10.2900.0"), "4.10.2900.0")
	writeInstalledCDM(t, filepath.Join(root, "4.10.2934.0"), "4.10.2934.0")

	if err := WriteHint(filepath.Join(root, HintFile), older); err != nil {
		t.Fatal(err)
	}

	found := Installed(Install{})
	if len(found) != 2 {
		t.Fatalf("found %d CDMs", len(found))
	}
	if found[0].Path != older {
		t.Fatalf("first = %q, want the hintedCDM directory %q", found[0].Path, older)
	}
}

func TestInstalledIgnoresStaleHint(t *testing.T) {
	home := testHome(t)

	root := filepath.Join(home, ".config", "net.imput.helium", "WidevineCdm")
	writeInstalledCDM(t, filepath.Join(root, "4.10.2900.0"), "4.10.2900.0")

	if err := WriteHint(filepath.Join(root, HintFile), filepath.Join(root, "9.9.9.9")); err != nil {
		t.Fatal(err)
	}

	found := Installed(Install{})
	if len(found) != 1 || found[0].Version != "4.10.2900.0" {
		t.Fatalf("found = %+v", found)
	}
}

func TestTargetRejectsUnsafeVersion(t *testing.T) {
	home := testHome(t)
	if err := os.MkdirAll(filepath.Join(home, ".config", "net.imput.helium"), 0o755); err != nil {
		t.Fatal(err)
	}

	for _, version := range []string{"../escape", "4.10/../../x", "..", ""} {
		if _, _, err := Target(Install{}, version); err == nil {
			t.Errorf("Target(%q) was accepted", version)
		}
	}

	target, hint, err := Target(Install{}, "4.10.2934.0")
	if err != nil {
		t.Fatalf("Target() = %v, want nil", err)
	}
	if want := filepath.Join(home, ".config", "net.imput.helium", "WidevineCdm", "4.10.2934.0"); target != want {
		t.Fatalf("target = %s, want %s", target, want)
	}
	if hint == "" {
		t.Fatal("expected a hint file on linux")
	}
}
