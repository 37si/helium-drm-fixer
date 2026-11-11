package helium

import (
	"os"
	"path/filepath"
	"testing"

	"hdf/internal/pathutil"
)

func fakeWindowsInstall(t *testing.T) (root, app string) {
	t.Helper()

	root = filepath.Join(t.TempDir(), "Helium")
	app = filepath.Join(root, "Application")

	for _, name := range []string{"chrome.exe", "chrome_proxy.exe"} {
		write(t, filepath.Join(app, name), "stub")
	}

	write(t, filepath.Join(app, "0.21.1.1", "chrome.dll"), "old")
	write(t, filepath.Join(app, "0.21.2.1", "chrome.dll"), "new")
	write(t, filepath.Join(app, "0.21.2.1", "WidevineCdm", "manifest.json"), `{"version":"4.10.2934.0"}`)
	write(t, filepath.Join(app, "0.21.2.1", "WidevineCdm", "_platform_specific", "win_x64", "widevinecdm.dll"), "so")

	return root, app
}

func write(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLayoutPicksTheNewestVersionDirectory(t *testing.T) {
	root, app := fakeWindowsInstall(t)

	install := layout(root, app, browserExe(app))

	if install.Root != root {
		t.Fatalf("root = %q", install.Root)
	}
	if want := filepath.Join(app, "0.21.2.1"); install.Dir != want {
		t.Fatalf("dir = %q, want %q", install.Dir, want)
	}
	if want := filepath.Join(app, "chrome.exe"); install.Exe != want {
		t.Fatalf("exe = %q, want %q", install.Exe, want)
	}
}

func TestBrowserExeFindsTheChromiumBinary(t *testing.T) {
	root, app := fakeWindowsInstall(t)

	if got := browserExe(app); got != filepath.Join(app, "chrome.exe") {
		t.Fatalf("exe = %q", got)
	}

	staged := filepath.Join(t.TempDir(), "Application")
	write(t, filepath.Join(staged, "new_chrome.exe"), "stub")

	if got := browserExe(staged); got != filepath.Join(staged, "new_chrome.exe") {
		t.Fatalf("exe = %q", got)
	}

	if got := browserExe(root); got != "" {
		t.Fatalf("the install root has no browser binary, got %q", got)
	}
}

func TestInstalledFindsTheBundledCDM(t *testing.T) {
	root, app := fakeWindowsInstall(t)

	found := cdmsUnder(filepath.Join(app, "0.21.2.1", "WidevineCdm"))
	if len(found) != 1 {
		t.Fatalf("found %d CDMs, want 1: %+v", len(found), found)
	}
	if found[0].Version != "4.10.2934.0" {
		t.Fatalf("version = %q", found[0].Version)
	}

	if found := cdmsUnder(filepath.Join(root, "Application", "WidevineCdm")); len(found) != 0 {
		t.Fatalf("the container was taken for a CDM: %+v", found)
	}
}

func TestDedupeKeepsTheOrderItWasGiven(t *testing.T) {
	list := []CDM{
		{Path: "/profile/4.10.2557.0", Version: "4.10.2557.0"},
		{Path: "/bundled/4.10.2934.0", Version: "4.10.2934.0"},
		{Path: "/profile/4.10.2557.0", Version: "4.10.2557.0"},
	}

	got := dedupe(list)
	if len(got) != 2 {
		t.Fatalf("got %d entries: %+v", len(got), got)
	}
	if got[0].Path != "/profile/4.10.2557.0" {
		t.Fatalf("the first entry should stay first: %+v", got)
	}
}

func TestLayoutWithoutAVersionDirectory(t *testing.T) {
	app := filepath.Join(t.TempDir(), "Application")
	write(t, filepath.Join(app, "chrome.exe"), "stub")

	install := layout(filepath.Dir(app), app, browserExe(app))
	if install.Dir != app {
		t.Fatalf("dir = %q, want the application directory", install.Dir)
	}
}

func TestVersionDirsReadTheApplicationDirectory(t *testing.T) {
	_, app := fakeWindowsInstall(t)

	versions := pathutil.VersionDirs(app)
	if len(versions) != 2 || versions[0] != "0.21.2.1" {
		t.Fatalf("versions = %v", versions)
	}
}

func TestLayoutFollowsAVersionSchemeChange(t *testing.T) {
	for _, version := range []string{"1.0.0", "0.22", "1.0.0.1"} {
		t.Run(version, func(t *testing.T) {
			app := filepath.Join(t.TempDir(), "Application")
			write(t, filepath.Join(app, "chrome.exe"), "stub")
			write(t, filepath.Join(app, version, "chrome.dll"), "build")

			install := layout(filepath.Dir(app), app, browserExe(app))
			if want := filepath.Join(app, version); install.Dir != want {
				t.Fatalf("dir = %q, want %q", install.Dir, want)
			}

			write(t, filepath.Join(app, version, "WidevineCdm", "manifest.json"), `{"version":"4.10.2934.0"}`)
			write(t, filepath.Join(app, version, "WidevineCdm", "_platform_specific", "win_x64", "widevinecdm.dll"), "so")

			if found := bundledCDMs(app); len(found) != 1 {
				t.Fatalf("found %d bundled CDMs under %q: %+v", len(found), version, found)
			}
		})
	}
}

func TestLayoutSkipsAStagedUpdateDirectory(t *testing.T) {
	app := filepath.Join(t.TempDir(), "Application")
	write(t, filepath.Join(app, "chrome.exe"), "stub")
	write(t, filepath.Join(app, "new_chrome.exe"), "staged")
	write(t, filepath.Join(app, "0.21.2.1", "chrome.dll"), "running")
	write(t, filepath.Join(app, "0.22.0.0", "empty.txt"), "downloaded, not applied")

	install := layout(filepath.Dir(app), app, browserExe(app))
	if want := filepath.Join(app, "0.21.2.1"); install.Dir != want {
		t.Fatalf("dir = %q, want the build that is actually there, %q", install.Dir, want)
	}
}

func TestCDMsAreFoundByContentNotByFolderName(t *testing.T) {
	root := t.TempDir()

	for _, name := range []string{"latest", "4.10.2934.0"} {
		dir := filepath.Join(root, name)
		write(t, filepath.Join(dir, "manifest.json"), `{"version":"4.10.2934.0"}`)
		write(t, filepath.Join(dir, "_platform_specific", "win_x64", "widevinecdm.dll"), "so")
	}

	found := cdmsUnder(root)
	if len(found) != 2 {
		t.Fatalf("found %d CDMs, want 2: %+v", len(found), found)
	}

	for _, item := range found {
		if item.Version != "4.10.2934.0" {
			t.Fatalf("version = %q for %q", item.Version, item.Path)
		}
	}
}

func TestAppDirHandlesAnUnversionedInstall(t *testing.T) {
	app := filepath.Join(t.TempDir(), "Helium", "Application")
	write(t, filepath.Join(app, "chrome.exe"), "stub")

	cases := []Install{
		{Dir: app, Exe: filepath.Join(app, "chrome.exe")},
		{Dir: app},
		{Dir: filepath.Join(app, "0.21.2.1")},
	}

	for _, install := range cases {
		if got := appDir(install); got != app {
			t.Fatalf("appDir(%+v) = %q, want %q", install, got, app)
		}
	}
}

func TestCDMsAreOrderedByVersionNotByName(t *testing.T) {
	root := t.TempDir()

	for _, version := range []string{"4.10.2900.0", "4.10.10000.0"} {
		dir := filepath.Join(root, version)
		write(t, filepath.Join(dir, "manifest.json"), `{"version":"`+version+`"}`)
		write(t, filepath.Join(dir, "_platform_specific", "win_x64", "widevinecdm.dll"), "so")
	}

	found := cdmsUnder(root)
	if len(found) != 2 {
		t.Fatalf("found %d CDMs: %+v", len(found), found)
	}
	if found[0].Version != "4.10.10000.0" {
		t.Fatalf("first = %q, want the newest", found[0].Version)
	}
}
