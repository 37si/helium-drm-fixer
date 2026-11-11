package browser

import (
	"os"
	"path/filepath"
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

func TestParse(t *testing.T) {
	tests := []struct {
		input string
		want  ID
	}{
		{"chrome", Chrome},
		{"Chrome", Chrome},
		{"google-chrome", Chrome},
		{"edge", Edge},
		{"Microsoft Edge", Edge},
		{"msedge", Edge},
		{"brave", Brave},
		{"vivaldi", Vivaldi},
		{"opera", Opera},
		{"opera-gx", OperaGX},
		{"Opera GX", OperaGX},
		{"operagx", OperaGX},
		{"opera gx", OperaGX},
		{"chromium", Chromium},
		{"ungoogled", UngoogledChromium},
		{"ungoogled-chromium", UngoogledChromium},
		{"fresh", Fresh},
		{"Fresh", Fresh},
		{"mozilla", Fresh},
		{"download", Fresh},
		{"ungoogled chromium", UngoogledChromium},
		{"  Brave  ", Brave},
	}

	for _, tc := range tests {
		got, ok := Parse(tc.input)
		if !ok {
			t.Fatalf("Parse(%q) failed", tc.input)
		}
		if got != tc.want {
			t.Fatalf("Parse(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestParseUnknown(t *testing.T) {
	for _, input := range []string{"firefox", "", "safari"} {
		if _, ok := Parse(input); ok {
			t.Fatalf("Parse(%q) should fail", input)
		}
	}
}

func TestAllHaveUniquePriorities(t *testing.T) {
	seen := make(map[int]ID, len(All))

	for _, b := range All {
		if other, ok := seen[b.Priority]; ok {
			t.Fatalf("%s and %s share priority %d", b.ID, other, b.Priority)
		}
		seen[b.Priority] = b.ID
	}
}

func TestDiscoverBundled(t *testing.T) {
	root := t.TempDir()
	cdmPath := filepath.Join(root, "131.0.6778.86", "WidevineCdm")
	writeCDM(t, cdmPath, "4.10.2891.0")

	location, ok := discover(paths{installRoots: []string{root}}, "")
	if !ok {
		t.Fatal("expected a bundled CDM")
	}
	if location.Path != cdmPath {
		t.Fatalf("path = %q", location.Path)
	}
	if location.Origin != OriginBundled {
		t.Fatalf("origin = %q", location.Origin)
	}
	if location.Version != "4.10.2891.0" {
		t.Fatalf("version = %q", location.Version)
	}
}

func TestDiscoverPrefersNewest(t *testing.T) {
	root := t.TempDir()
	older := filepath.Join(root, "120.0.0.1", "WidevineCdm")
	newer := filepath.Join(root, "131.0.6778.86", "WidevineCdm")
	writeCDM(t, older, "4.10.2800.0")
	writeCDM(t, newer, "4.10.2891.0")

	location, ok := discover(paths{installRoots: []string{root}}, "")
	if !ok {
		t.Fatal("expected a bundled CDM")
	}
	if location.Path != newer {
		t.Fatalf("path = %q, want %q", location.Path, newer)
	}
}

func TestDiscoverComponentUpdater(t *testing.T) {
	userData := t.TempDir()
	cdmPath := filepath.Join(userData, "WidevineCdm", "4.10.2891.0")
	writeCDM(t, cdmPath, "4.10.2891.0")

	location, ok := discover(paths{userDataDirs: []string{userData}}, "")
	if !ok {
		t.Fatal("expected a component updater CDM")
	}
	if location.Origin != OriginComponentUpdater {
		t.Fatalf("origin = %q", location.Origin)
	}
	if location.Path != cdmPath {
		t.Fatalf("path = %q", location.Path)
	}
}

func TestDiscoverPrefersBundledOverComponentUpdater(t *testing.T) {
	root := t.TempDir()
	userData := t.TempDir()

	bundled := filepath.Join(root, "131.0.6778.86", "WidevineCdm")
	updater := filepath.Join(userData, "WidevineCdm", "4.10.2891.0")
	writeCDM(t, bundled, "4.10.2891.0")
	writeCDM(t, updater, "4.10.2891.0")

	location, ok := discover(paths{installRoots: []string{root}, userDataDirs: []string{userData}}, "")
	if !ok {
		t.Fatal("expected a CDM")
	}
	if location.Path != bundled {
		t.Fatalf("path = %q, want the bundled copy", location.Path)
	}
}

func TestDiscoverNothing(t *testing.T) {
	if _, ok := discover(paths{installRoots: []string{filepath.Join(t.TempDir(), "missing")}}, ""); ok {
		t.Fatal("expected no CDM")
	}
}

func TestLocateBrowserExecutable(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "Application")
	exe := filepath.Join(app, "chrome.exe")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}

	cdmPath := filepath.Join(app, "131.0.6778.86", "WidevineCdm")
	writeCDM(t, cdmPath, "4.10.2891.0")

	location, ok := Locate(exe)
	if !ok {
		t.Fatal("expected the executable to lead to its CDM")
	}
	if location.Path != cdmPath {
		t.Fatalf("path = %q, want %q", location.Path, cdmPath)
	}
}

func TestLocateBrowserInstallRoot(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "Application")
	cdmPath := filepath.Join(app, "131.0.6778.86", "WidevineCdm")
	writeCDM(t, cdmPath, "4.10.2891.0")

	location, ok := Locate(root)
	if !ok {
		t.Fatal("expected the install root to lead to its CDM")
	}
	if location.Path != cdmPath {
		t.Fatalf("path = %q, want %q", location.Path, cdmPath)
	}
}

func TestLocateAppBundle(t *testing.T) {
	bundle := filepath.Join(t.TempDir(), "Helium.app")
	exe := filepath.Join(bundle, "Contents", "MacOS", "Helium")
	if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}

	cdmPath := filepath.Join(bundle, "Contents", "Frameworks", "Helium Framework.framework", "Versions", "131.0.6778.86", "Libraries", "WidevineCdm")
	writeCDM(t, cdmPath, "4.10.2891.0")

	location, ok := Locate(bundle)
	if !ok {
		t.Fatal("expected the app bundle to lead to its CDM")
	}
	if location.Path != cdmPath {
		t.Fatalf("path = %q, want %q", location.Path, cdmPath)
	}

	location, ok = Locate(exe)
	if !ok || location.Path != cdmPath {
		t.Fatalf("executable lookup = %+v, %v", location, ok)
	}
}

func TestInstallRootsUsesExecutableDir(t *testing.T) {
	base := filepath.Join(string(filepath.Separator), "opt")
	exe := filepath.Join(base, "vivaldi", "vivaldi")

	roots := installRoots(paths{}, exe)

	if len(roots) != 2 || roots[0] != filepath.Join(base, "vivaldi") || roots[1] != base {
		t.Fatalf("roots = %v", roots)
	}
}

func TestInstallRootsDeduplicates(t *testing.T) {
	base := filepath.Join(string(filepath.Separator), "opt", "vivaldi")
	roots := installRoots(paths{installRoots: []string{base}}, filepath.Join(base, "vivaldi"))

	if len(roots) != 2 {
		t.Fatalf("roots = %v", roots)
	}
}

func TestOnPath(t *testing.T) {
	if !onPath("/usr/bin/google-chrome") {
		t.Fatal("/usr/bin should be on PATH")
	}
	if onPath("/opt/google/chrome/google-chrome") {
		t.Fatal("/opt/google/chrome is not a PATH directory")
	}
}

func TestFindExecutableIgnoresSiblingOnPath(t *testing.T) {
	if got := findExecutable(paths{executables: []string{"/opt/chrome-beta/google-chrome"}}); got != "" {
		t.Fatalf("findExecutable = %q, want empty", got)
	}
}

func TestDetectOneWithInjectedPaths(t *testing.T) {
	root := t.TempDir()
	cdmPath := filepath.Join(root, "131.0.6778.86", "WidevineCdm")
	writeCDM(t, cdmPath, "4.10.2891.0")

	original := platformPaths[Brave]
	platformPaths[Brave] = paths{installRoots: []string{root}}
	t.Cleanup(func() { platformPaths[Brave] = original })

	detection, ok := DetectOne(t.Context(), Brave)
	if !ok {
		t.Fatal("expected Brave to be detected")
	}
	if !detection.Available() {
		t.Fatal("expected the CDM to be available")
	}
	if detection.CDM.Path != cdmPath {
		t.Fatalf("path = %q", detection.CDM.Path)
	}
}

func TestDetectOneUnknown(t *testing.T) {
	if _, ok := DetectOne(t.Context(), ID("netscape")); ok {
		t.Fatal("unknown ids should not be detected")
	}
}
