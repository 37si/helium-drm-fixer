package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const MarkerFile = ".hdf.json"

const (
	MethodCopy     = "copy"
	MethodDownload = "download"
	MethodRollback = "rollback"
)

const maxRecords = 200

type Record struct {
	Target      string    `json:"target"`
	Version     string    `json:"version"`
	Source      string    `json:"source"`
	SourceID    string    `json:"source_id,omitempty"`
	SourcePath  string    `json:"source_path,omitempty"`
	Tool        string    `json:"tool"`
	Method      string    `json:"method"`
	InstalledAt time.Time `json:"installed_at"`
	DryRun      bool      `json:"dry_run,omitempty"`
	Backup      string    `json:"backup,omitempty"`
	Verified    bool      `json:"verified,omitempty"`
}

type ledger struct {
	Version int      `json:"version"`
	Records []Record `json:"records"`
}

type Store struct {
	Dir string
}

func DefaultDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "hdf")
}

func Open(dir string) Store {
	if dir == "" {
		dir = DefaultDir()
	}
	return Store{Dir: dir}
}

func (s Store) Path() string {
	if s.Dir == "" {
		return ""
	}
	return filepath.Join(s.Dir, "history.json")
}

func (s Store) Load() ([]Record, error) {
	if s.Dir == "" {
		return nil, nil
	}

	raw, err := os.ReadFile(s.Path())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var stored ledger
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, err
	}

	return stored.Records, nil
}

func (s Store) Append(record Record) error {
	records, err := s.Load()
	if err != nil {
		return err
	}

	if record.InstalledAt.IsZero() {
		record.InstalledAt = time.Now()
	}

	records = append(records, record)
	if len(records) > maxRecords {
		records = records[len(records)-maxRecords:]
	}

	return s.save(records)
}

func (s Store) MarkVerified(target string) error {
	records, err := s.Load()
	if err != nil {
		return err
	}

	for i := len(records) - 1; i >= 0; i-- {
		if records[i].Target == target {
			records[i].Verified = true
			return s.save(records)
		}
	}

	return nil
}

func (s Store) Latest(target string) (Record, bool) {
	records, err := s.Load()
	if err != nil {
		return Record{}, false
	}

	for i := len(records) - 1; i >= 0; i-- {
		if records[i].Target == target {
			return records[i], true
		}
	}

	return Record{}, false
}

func (s Store) save(records []Record) error {
	if s.Dir == "" {
		return nil
	}

	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return err
	}

	payload, err := json.MarshalIndent(ledger{Version: 1, Records: records}, "", "  ")
	if err != nil {
		return err
	}

	file, err := os.CreateTemp(s.Dir, "history-*.tmp")
	if err != nil {
		return err
	}

	name := file.Name()
	defer os.Remove(name)

	if _, err := file.Write(append(payload, '\n')); err != nil {
		file.Close()
		return err
	}

	if err := file.Chmod(0o644); err != nil {
		file.Close()
		return err
	}

	if err := file.Close(); err != nil {
		return err
	}

	return os.Rename(name, s.Path())
}

func WriteMarker(target string, record Record) error {
	payload, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(target, MarkerFile), append(payload, '\n'), 0o644)
}

func ReadMarker(target string) (Record, bool) {
	raw, err := os.ReadFile(filepath.Join(target, MarkerFile))
	if err != nil {
		return Record{}, false
	}

	var record Record
	if err := json.Unmarshal(raw, &record); err != nil {
		return Record{}, false
	}

	return record, true
}
