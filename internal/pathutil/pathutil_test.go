package pathutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpand(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("home dir: %v", err)
	}

	t.Setenv("HELIUM_TEST_ROOT", "/opt/test")

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"home", "~/Library/Application Support", filepath.Join(home, "Library/Application Support")},
		{"bare tilde", "~", home},
		{"env var", "${HELIUM_TEST_ROOT}/chrome", "/opt/test/chrome"},
		{"windows var", `%HELIUM_TEST_ROOT%\Chrome`, `/opt/test\Chrome`},
		{"plain", "/usr/bin/chromium", "/usr/bin/chromium"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := Expand(tc.input)
			if !ok {
				t.Fatalf("Expand(%q) reported a missing variable", tc.input)
			}
			if got != tc.want {
				t.Fatalf("Expand(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestExpandMissingVariable(t *testing.T) {
	os.Unsetenv("HELIUM_TEST_MISSING")

	if _, ok := Expand(`%HELIUM_TEST_MISSING%\Chrome`); ok {
		t.Fatal("expected the unset variable to be reported")
	}

	if _, ok := Expand("${HELIUM_TEST_MISSING}/x"); ok {
		t.Fatal("expected the unset variable to be reported")
	}
}

func TestExpandAllDropsUnset(t *testing.T) {
	t.Setenv("HELIUM_TEST_ROOT", "/opt/test")

	got := ExpandAll([]string{"${HELIUM_TEST_ROOT}/a", "${HELIUM_TEST_MISSING}/b"})
	if len(got) != 1 || got[0] != "/opt/test/a" {
		t.Fatalf("ExpandAll = %v", got)
	}
}

func TestVersionDirs(t *testing.T) {
	root := t.TempDir()

	for _, name := range []string{"120.0.0.1", "131.0.6778.86", "9.0.0.0", "not-a-version", "1.2.3"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	got := VersionDirs(root)
	want := []string{"131.0.6778.86", "120.0.0.1", "9.0.0.0", "1.2.3"}

	if len(got) != len(want) {
		t.Fatalf("VersionDirs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("VersionDirs = %v, want %v", got, want)
		}
	}
}

func TestVersionDirsMissingRoot(t *testing.T) {
	if got := VersionDirs(filepath.Join(t.TempDir(), "missing")); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestIsVersion(t *testing.T) {
	for _, name := range []string{"131.0.6778.86", "9.0.0.0", "131.0.6778", "1.0.0", "0.22"} {
		if !IsVersion(name) {
			t.Fatalf("%q should be a version", name)
		}
	}

	for _, name := range []string{
		"131.0.6778.86.1",
		"v131.0.0.0",
		"131.0.6778-beta",
		"1",
		"1..2",
		"-1.0.0",
		"WidevineCdm",
		"Installer",
		"",
	} {
		if IsVersion(name) {
			t.Fatalf("%q should not be a version", name)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	if CompareVersions("131.0.0.0", "9.0.0.0") <= 0 {
		t.Fatal("131 should sort after 9")
	}
	if CompareVersions("9.0.0.0", "131.0.0.0") >= 0 {
		t.Fatal("9 should sort before 131")
	}
	if CompareVersions("131.0.6778.86", "131.0.6778.86") != 0 {
		t.Fatal("identical versions should compare equal")
	}
	if CompareVersions("1.2", "1.2.0") != 0 {
		t.Fatal("a missing component should count as zero")
	}
	if CompareVersions("1.2.1", "1.2") <= 0 {
		t.Fatal("1.2.1 should be newer than 1.2")
	}
	if CompareVersions("1.2", "1.2.1") >= 0 {
		t.Fatal("1.2 should be older than 1.2.1")
	}
}

func TestMigrateDirAdoptsTheLegacyDirectory(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, LegacyDirName)

	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "history.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(root, "hdf")

	if !MigrateDir(target) {
		t.Fatal("the legacy directory was not adopted")
	}

	if _, err := os.Stat(filepath.Join(target, "history.json")); err != nil {
		t.Fatalf("history did not move: %v", err)
	}

	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatal("the legacy directory is still there")
	}
}

func TestMigrateDirLeavesAnExistingDirectoryAlone(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "hdf")

	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, LegacyDirName), 0o755); err != nil {
		t.Fatal(err)
	}

	if MigrateDir(target) {
		t.Fatal("migration ran over an existing directory")
	}
}

func TestMigrateDirIgnoresAMissingLegacyDirectory(t *testing.T) {
	if MigrateDir(filepath.Join(t.TempDir(), "hdf")) {
		t.Fatal("migration ran without a legacy directory")
	}

	if MigrateDir("") {
		t.Fatal("migration ran on an empty path")
	}
}
