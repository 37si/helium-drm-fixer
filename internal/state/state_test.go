package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAppendAndLatest(t *testing.T) {
	store := Open(t.TempDir())

	for _, version := range []string{"4.10.1.0", "4.10.2.0"} {
		if err := store.Append(Record{Target: "/tmp/cdm", Version: version, Source: "Brave"}); err != nil {
			t.Fatal(err)
		}
	}

	record, ok := store.Latest("/tmp/cdm")
	if !ok {
		t.Fatal("no record for the target")
	}
	if record.Version != "4.10.2.0" {
		t.Fatalf("version = %q, want the newest", record.Version)
	}
	if record.InstalledAt.IsZero() {
		t.Fatal("InstalledAt was not filled in")
	}

	if _, ok := store.Latest("/tmp/other"); ok {
		t.Fatal("matched a target that was never recorded")
	}
}

func TestLoadMissingHistoryIsEmpty(t *testing.T) {
	records, err := Open(t.TempDir()).Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 0 {
		t.Fatalf("records = %d", len(records))
	}
}

func TestHistoryIsBounded(t *testing.T) {
	store := Open(t.TempDir())

	for i := 0; i < maxRecords+5; i++ {
		if err := store.Append(Record{Target: "/tmp/cdm", Version: "4.10.1.0"}); err != nil {
			t.Fatal(err)
		}
	}

	records, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != maxRecords {
		t.Fatalf("records = %d, want %d", len(records), maxRecords)
	}
}

func TestMarkVerified(t *testing.T) {
	store := Open(t.TempDir())

	if err := store.Append(Record{Target: "/tmp/cdm", Version: "4.10.1.0"}); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkVerified("/tmp/cdm"); err != nil {
		t.Fatal(err)
	}

	record, ok := store.Latest("/tmp/cdm")
	if !ok || !record.Verified {
		t.Fatalf("record = %+v", record)
	}
}

func TestAppendKeepsCorruptHistory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Open(dir).Append(Record{Target: "/tmp/cdm"}); err == nil {
		t.Fatal("expected the corrupt history to be reported")
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "{not json" {
		t.Fatalf("history was overwritten: %q", raw)
	}
}

func TestMarkerRoundTrip(t *testing.T) {
	target := t.TempDir()
	want := Record{
		Target:      target,
		Version:     "4.10.3112.0",
		Source:      "Brave",
		Tool:        "hdf 3.0.0",
		Method:      MethodCopy,
		InstalledAt: time.Now().Truncate(time.Second),
	}

	if err := WriteMarker(target, want); err != nil {
		t.Fatal(err)
	}

	got, ok := ReadMarker(target)
	if !ok {
		t.Fatal("marker was not found")
	}
	if got.Version != want.Version || got.Source != want.Source {
		t.Fatalf("marker = %+v", got)
	}
	if !got.InstalledAt.Equal(want.InstalledAt) {
		t.Fatalf("time = %v, want %v", got.InstalledAt, want.InstalledAt)
	}

	if _, err := os.Stat(filepath.Join(target, MarkerFile)); err != nil {
		t.Fatal(err)
	}
}

func TestReadMarkerMissing(t *testing.T) {
	if _, ok := ReadMarker(t.TempDir()); ok {
		t.Fatal("found a marker that was never written")
	}
}
