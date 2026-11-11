package fixer

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"charm.land/log/v2"

	"hdf/internal/backup"
	"hdf/internal/browser"
	"hdf/internal/cdm"
	"hdf/internal/helium"
	"hdf/internal/state"
)

func writeCDM(t *testing.T, dir, version string) string {
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

func discardLogger() *log.Logger { return log.New(io.Discard) }

func heliumUserData(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)

	if runtime.GOOS == "windows" {
		local := t.TempDir()
		t.Setenv("LOCALAPPDATA", local)

		return filepath.Join(local, "imput", "Helium", "User Data")
	}

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	return filepath.Join(home, ".config", "net.imput.helium")
}

func TestResolveCustomPathDirect(t *testing.T) {
	cdm := writeCDM(t, filepath.Join(t.TempDir(), "WidevineCdm"), "4.10.2891.0")

	got, err := resolveCustomPath(cdm)
	if err != nil {
		t.Fatal(err)
	}
	if got != cdm {
		t.Fatalf("resolveCustomPath = %q, want %q", got, cdm)
	}
}

func TestResolveCustomPathInstallDir(t *testing.T) {
	root := t.TempDir()
	cdm := writeCDM(t, filepath.Join(root, "131.0.6778.86", "WidevineCdm"), "4.10.2891.0")

	got, err := resolveCustomPath(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != cdm {
		t.Fatalf("resolveCustomPath = %q, want %q", got, cdm)
	}
}

func TestResolveCustomPathPlainCDMDir(t *testing.T) {
	root := t.TempDir()
	cdm := writeCDM(t, filepath.Join(root, "WidevineCdm"), "4.10.2891.0")

	got, err := resolveCustomPath(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != cdm {
		t.Fatalf("resolveCustomPath = %q, want %q", got, cdm)
	}
}

func TestResolveCustomPathInvalid(t *testing.T) {
	if _, err := resolveCustomPath(filepath.Join(t.TempDir(), "missing")); !errors.Is(err, ErrNoSource) {
		t.Fatalf("expected ErrNoSource, got %v", err)
	}

	if _, err := resolveCustomPath(t.TempDir()); !errors.Is(err, ErrNoSource) {
		t.Fatalf("expected ErrNoSource for an empty directory, got %v", err)
	}
}

func TestResolveCustomPathBrowserExecutable(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "Application")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}

	exe := filepath.Join(app, "chrome.exe")
	if err := os.WriteFile(exe, []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}

	cdm := writeCDM(t, filepath.Join(app, "131.0.6778.86", "WidevineCdm"), "4.10.2891.0")

	got, err := resolveCustomPath(exe)
	if err != nil {
		t.Fatal(err)
	}
	if got != cdm {
		t.Fatalf("resolveCustomPath = %q, want %q", got, cdm)
	}
}

func TestResolveCustomPathBrowserInstallRoot(t *testing.T) {
	root := t.TempDir()
	cdm := writeCDM(t, filepath.Join(root, "Application", "131.0.6778.86", "WidevineCdm"), "4.10.2891.0")

	got, err := resolveCustomPath(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != cdm {
		t.Fatalf("resolveCustomPath = %q, want %q", got, cdm)
	}
}

func TestResolveWithCustomSource(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("macOS installs into the app bundle instead of the user data directory")
	}

	userData := heliumUserData(t)
	if err := os.MkdirAll(userData, 0o755); err != nil {
		t.Fatal(err)
	}

	heliumRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(heliumRoot, "131.0.6778.86"), 0o755); err != nil {
		t.Fatal(err)
	}

	cdm := writeCDM(t, filepath.Join(t.TempDir(), "WidevineCdm"), "4.10.2891.0")

	plan, err := Resolve(t.Context(), Options{SourcePath: cdm, HeliumPath: heliumRoot}, discardLogger(), nil)
	if err != nil {
		t.Fatal(err)
	}

	if plan.Source.Path != cdm {
		t.Fatalf("source = %q", plan.Source.Path)
	}
	if plan.Source.Version != "4.10.2891.0" {
		t.Fatalf("version = %q", plan.Source.Version)
	}

	wantTarget := filepath.Join(userData, "WidevineCdm", "4.10.2891.0")
	if plan.Target != wantTarget {
		t.Fatalf("target = %q, want %q", plan.Target, wantTarget)
	}
	if plan.HintFile == "" {
		t.Fatal("expected a hint file on linux")
	}
}

func TestResolveUnknownBrowser(t *testing.T) {
	_, err := Resolve(t.Context(), Options{Browser: "netscape"}, discardLogger(), nil)
	if !errors.Is(err, ErrNoSource) {
		t.Fatalf("expected ErrNoSource, got %v", err)
	}
}

func TestApply(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	userData := filepath.Join(home, ".config", "net.imput.helium")
	if err := os.MkdirAll(userData, 0o755); err != nil {
		t.Fatal(err)
	}

	source := writeCDM(t, filepath.Join(t.TempDir(), "WidevineCdm"), "4.10.2891.0")
	target := filepath.Join(userData, "WidevineCdm", "4.10.2891.0")
	hint := filepath.Join(userData, "WidevineCdm", "latest-component-updated-widevine-cdm")

	plan := Plan{
		Source:   Source{Name: "test", Path: source, Version: "4.10.2891.0"},
		Target:   target,
		HintFile: hint,
	}

	var steps []string
	result, err := Apply(t.Context(), plan, discardLogger(), func(step string, _ float64) {
		steps = append(steps, step)
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Target != target {
		t.Fatalf("result target = %q", result.Target)
	}

	raw, err := os.ReadFile(filepath.Join(target, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"version":"4.10.2891.0"}` {
		t.Fatalf("manifest = %s", raw)
	}

	if _, err := os.Stat(hint); err != nil {
		t.Fatalf("hint file: %v", err)
	}

	if len(steps) < 2 || steps[0] != "copying WidevineCdm" {
		t.Fatalf("steps = %v", steps)
	}
}

func TestApplyFailsOnInvalidSource(t *testing.T) {
	plan := Plan{Source: Source{Path: t.TempDir()}, Target: filepath.Join(t.TempDir(), "dst")}

	if _, err := Apply(t.Context(), plan, discardLogger(), nil); err == nil {
		t.Fatal("expected an error")
	}
}

func TestApplyKeepsBackupAndRecords(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	userData := filepath.Join(home, ".config", "net.imput.helium")
	if err := os.MkdirAll(userData, 0o755); err != nil {
		t.Fatal(err)
	}

	target := writeCDM(t, filepath.Join(userData, "WidevineCdm", "4.10.2900.0"), "4.10.2900.0")
	source := writeCDM(t, filepath.Join(t.TempDir(), "WidevineCdm"), "4.10.3112.0")

	backupDir := t.TempDir()
	stateDir := t.TempDir()

	plan := Plan{
		Source:    Source{Name: "Brave", BrowserID: browser.Brave, Path: source, Version: "4.10.3112.0"},
		Target:    target,
		Backup:    true,
		BackupDir: backupDir,
		Backups:   5,
		StateDir:  stateDir,
		Tool:      "hdf test",
	}

	result, err := Apply(t.Context(), plan, discardLogger(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Backup == "" {
		t.Fatal("no backup was recorded")
	}
	if result.Version != "4.10.3112.0" || result.Source != "Brave" {
		t.Fatalf("result = %+v", result)
	}

	snapshots, err := backup.New(backupDir, 5).List()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("snapshots = %d", len(snapshots))
	}
	if snapshots[0].Version != "4.10.2900.0" {
		t.Fatalf("backup holds %q, want the replaced version", snapshots[0].Version)
	}

	marker, ok := state.ReadMarker(target)
	if !ok {
		t.Fatal("no marker inside the installed CDM")
	}
	if marker.Version != "4.10.3112.0" || marker.Source != "Brave" || marker.Tool != "hdf test" {
		t.Fatalf("marker = %+v", marker)
	}

	records, err := state.Open(stateDir).Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Method != state.MethodCopy {
		t.Fatalf("records = %+v", records)
	}
}

func TestRollbackRestoresSnapshot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	target := writeCDM(t, filepath.Join(t.TempDir(), "WidevineCdm"), "4.10.2900.0")
	backupDir := t.TempDir()
	stateDir := t.TempDir()

	store := backup.New(backupDir, 5)
	snapshot, err := store.Create(target)
	if err != nil {
		t.Fatal(err)
	}

	if err := cdm.Copy(writeCDM(t, filepath.Join(t.TempDir(), "WidevineCdm"), "4.10.3112.0"), target); err != nil {
		t.Fatal(err)
	}

	result, err := Rollback(snapshot, Plan{BackupDir: backupDir, Backups: 5, StateDir: stateDir, Tool: "test"}, discardLogger(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Version != "4.10.2900.0" {
		t.Fatalf("version = %q", result.Version)
	}

	version, err := cdm.Version(target)
	if err != nil {
		t.Fatal(err)
	}
	if version != "4.10.2900.0" {
		t.Fatalf("target holds %q", version)
	}

	records, err := state.Open(stateDir).Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Method != state.MethodRollback {
		t.Fatalf("records = %+v", records)
	}
}

func TestPickSourceNewest(t *testing.T) {
	detections := []browser.Detection{
		{
			Browser: browser.Browser{ID: browser.Chrome, Name: "Google Chrome", Priority: 10},
			CDM:     &browser.Location{Path: "/opt/chrome/WidevineCdm", Version: "4.10.2900.0"},
		},
		{
			Browser: browser.Browser{ID: browser.Brave, Name: "Brave", Priority: 30},
			CDM:     &browser.Location{Path: "/opt/brave/WidevineCdm", Version: "4.10.3112.0"},
		},
	}

	first, err := pickSource(t.Context(), Options{}, detections, discardLogger(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.Name != "Google Chrome" {
		t.Fatalf("default source = %q", first.Name)
	}

	newest, err := pickSource(t.Context(), Options{Newest: true}, detections, discardLogger(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if newest.Name != "Brave" {
		t.Fatalf("newest source = %q", newest.Name)
	}
}

func TestApplyBacksUpActiveCDMForNewVersion(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("macOS installs into the app bundle instead of the user data directory")
	}

	root := filepath.Join(heliumUserData(t), "WidevineCdm")
	active := writeCDM(t, filepath.Join(root, "4.10.2900.0"), "4.10.2900.0")

	hint := filepath.Join(root, helium.HintFile)
	if err := helium.WriteHint(hint, active); err != nil {
		t.Fatal(err)
	}

	source := writeCDM(t, filepath.Join(t.TempDir(), "WidevineCdm"), "4.10.2934.0")
	backupDir := t.TempDir()
	stateDir := t.TempDir()

	plan := Plan{
		Source:    Source{Name: "Google Chrome", Path: source, Version: "4.10.2934.0"},
		Helium:    helium.Install{Root: root, Dir: root},
		Target:    filepath.Join(root, "4.10.2934.0"),
		HintFile:  hint,
		Backup:    true,
		BackupDir: backupDir,
		Backups:   5,
		StateDir:  stateDir,
		Tool:      "test",
	}

	result, err := Apply(t.Context(), plan, discardLogger(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Backup == "" {
		t.Fatal("the active CDM was not backed up")
	}

	snapshots, err := backup.New(backupDir, 5).List()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("snapshots = %d", len(snapshots))
	}
	if snapshots[0].Version != "4.10.2900.0" || snapshots[0].Target != active {
		t.Fatalf("snapshot = %+v", snapshots[0])
	}

	if got := hinted(t, hint); got != plan.Target {
		t.Fatalf("hint points at %q, want the new directory", got)
	}

	if _, err := Rollback(snapshots[0], plan, discardLogger(), nil); err != nil {
		t.Fatal(err)
	}

	if got := hinted(t, hint); got != active {
		t.Fatalf("after rollback the hint points at %q, want %q", got, active)
	}
}

func hinted(t *testing.T, path string) string {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var payload struct {
		Path string `json:"Path"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}

	return payload.Path
}
