package backup

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
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

func TestCreateAndRestore(t *testing.T) {
	target := writeCDM(t, filepath.Join(t.TempDir(), "WidevineCdm"), "4.10.2900.0")
	store := New(filepath.Join(t.TempDir(), "backups"), 5)

	snapshot, err := store.Create(target)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ID == "" {
		t.Fatal("no snapshot was taken")
	}
	if snapshot.Version != "4.10.2900.0" {
		t.Fatalf("version = %q", snapshot.Version)
	}
	if snapshot.Files != 2 || snapshot.Bytes == 0 {
		t.Fatalf("measured %d files, %d bytes", snapshot.Files, snapshot.Bytes)
	}

	if err := os.WriteFile(filepath.Join(target, "manifest.json"), []byte(`{"version":"4.10.3112.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	found, err := store.Latest()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Restore(found); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(target, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"version":"4.10.2900.0"}` {
		t.Fatalf("manifest = %s", raw)
	}
}

func TestCreateSkipsMissingTarget(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "backups"), 5)

	snapshot, err := store.Create(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ID != "" {
		t.Fatalf("took a snapshot of nothing: %+v", snapshot)
	}
}

func TestListIsNewestFirst(t *testing.T) {
	target := writeCDM(t, filepath.Join(t.TempDir(), "WidevineCdm"), "4.10.2900.0")
	store := New(filepath.Join(t.TempDir(), "backups"), 5)

	first, err := store.Create(target)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Create(target)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID == first.ID {
		t.Fatal("two snapshots share an id")
	}

	snapshots, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("snapshots = %d", len(snapshots))
	}
	if snapshots[0].ID != second.ID {
		t.Fatalf("newest is %q, want %q", snapshots[0].ID, second.ID)
	}
}

func TestGetResolvesLatestAndPrefix(t *testing.T) {
	target := writeCDM(t, filepath.Join(t.TempDir(), "WidevineCdm"), "4.10.2900.0")
	store := New(filepath.Join(t.TempDir(), "backups"), 5)

	snapshot, err := store.Create(target)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.Get("latest"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(snapshot.ID[:8]); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("nope"); !errors.Is(err, ErrNoSnapshots) {
		t.Fatalf("expected ErrNoSnapshots, got %v", err)
	}
}

func TestPruneKeepsNewest(t *testing.T) {
	target := writeCDM(t, filepath.Join(t.TempDir(), "WidevineCdm"), "4.10.2900.0")
	store := New(filepath.Join(t.TempDir(), "backups"), 2)

	for i := 0; i < 4; i++ {
		if _, err := store.Create(target); err != nil {
			t.Fatal(err)
		}
	}

	snapshots, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("snapshots = %d, want 2", len(snapshots))
	}
}

func TestPruneRemovesDirectoriesWithoutMetadata(t *testing.T) {
	target := writeCDM(t, filepath.Join(t.TempDir(), "WidevineCdm"), "4.10.2900.0")
	dir := filepath.Join(t.TempDir(), "backups")
	store := New(dir, 1)

	if _, err := store.Create(target); err != nil {
		t.Fatal(err)
	}

	broken := filepath.Join(dir, "19990101-000000.000")
	if err := os.MkdirAll(broken, 0o755); err != nil {
		t.Fatal(err)
	}

	store.Prune()

	if _, err := os.Stat(broken); !os.IsNotExist(err) {
		t.Fatalf("a directory without metadata was kept: %v", err)
	}
}

func TestClearRemovesEverySnapshot(t *testing.T) {
	target := writeCDM(t, filepath.Join(t.TempDir(), "WidevineCdm"), "4.10.2900.0")
	dir := filepath.Join(t.TempDir(), "backups")
	store := New(dir, 5)

	for i := 0; i < 3; i++ {
		if _, err := store.Create(target); err != nil {
			t.Fatal(err)
		}
	}

	count, bytes, err := store.Clear()
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 || bytes == 0 {
		t.Fatalf("cleared %d snapshots, %d bytes", count, bytes)
	}

	snapshots, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 0 {
		t.Fatalf("snapshots left behind: %d", len(snapshots))
	}
}

func TestDisabledStoreDoesNothing(t *testing.T) {
	store := New("", 5)

	snapshot, err := store.Create(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ID != "" {
		t.Fatal("a disabled store took a snapshot")
	}

	if _, err := store.Latest(); !errors.Is(err, ErrNoSnapshots) {
		t.Fatalf("expected ErrNoSnapshots, got %v", err)
	}
}

func TestRestoreRejectsForeignTarget(t *testing.T) {
	dir := t.TempDir()
	store := New(filepath.Join(dir, "backups"), 3)

	target := filepath.Join(dir, "WidevineCdm")
	writeCDM(t, target, "4.10.2934.0")

	snapshot, err := store.Create(target)
	if err != nil {
		t.Fatal(err)
	}

	victim := filepath.Join(dir, "precious")
	if err := os.MkdirAll(victim, 0o755); err != nil {
		t.Fatal(err)
	}

	tampered := snapshot
	tampered.Target = victim
	if err := store.Restore(tampered); !errors.Is(err, ErrBadTarget) {
		t.Fatalf("Restore() = %v, want ErrBadTarget", err)
	}
	if _, err := os.Stat(victim); err != nil {
		t.Fatalf("the refused restore removed %s", victim)
	}

	relative := snapshot
	relative.Target = "../../etc"
	if err := store.Restore(relative); !errors.Is(err, ErrBadTarget) {
		t.Fatalf("Restore(relative) = %v, want ErrBadTarget", err)
	}
}
