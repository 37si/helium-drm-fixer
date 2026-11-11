package backup

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"hdf/internal/cdm"
)

var ErrNoSnapshots = errors.New("no backup to restore")

var ErrBadTarget = errors.New("snapshot target is not a WidevineCdm directory")

type Snapshot struct {
	ID      string    `json:"id"`
	Target  string    `json:"target"`
	Version string    `json:"version"`
	Created time.Time `json:"created"`
	Files   int       `json:"files"`
	Bytes   int64     `json:"bytes"`
	Dir     string    `json:"-"`
}

type Store struct {
	Dir  string
	Keep int
}

const metaFile = "meta.json"

const DefaultKeep = 5

func New(dir string, keep int) Store {
	if keep <= 0 {
		keep = DefaultKeep
	}
	return Store{Dir: dir, Keep: keep}
}

func (s Store) Enabled() bool { return s.Dir != "" }

func (s Store) Create(target string) (Snapshot, error) {
	if !s.Enabled() || !cdm.Valid(target) {
		return Snapshot{}, nil
	}

	created := time.Now()

	dir, id, err := s.reserve(created)
	if err != nil {
		return Snapshot{}, err
	}

	stored := filepath.Join(dir, "cdm")
	if err := cdm.Copy(target, stored); err != nil {
		os.RemoveAll(dir)
		return Snapshot{}, fmt.Errorf("copy to backup: %w", err)
	}

	snapshot := Snapshot{ID: id, Target: target, Created: created, Dir: dir}
	snapshot.Version, _ = cdm.Version(stored)
	snapshot.Files, snapshot.Bytes = measure(stored)

	if err := writeMeta(dir, snapshot); err != nil {
		os.RemoveAll(dir)
		return Snapshot{}, err
	}

	s.Prune()

	return snapshot, nil
}

func (s Store) reserve(created time.Time) (string, string, error) {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return "", "", fmt.Errorf("create backup dir: %w", err)
	}

	base := created.Format("20060102-150405.000")

	for attempt := 1; attempt <= 100; attempt++ {
		id := base
		if attempt > 1 {
			id = fmt.Sprintf("%s-%d", base, attempt)
		}

		dir := filepath.Join(s.Dir, id)
		err := os.Mkdir(dir, 0o755)
		switch {
		case err == nil:
			return dir, id, nil
		case !os.IsExist(err):
			return "", "", fmt.Errorf("create backup dir: %w", err)
		}
	}

	return "", "", errors.New("could not reserve a backup directory")
}

func (s Store) List() ([]Snapshot, error) {
	if !s.Enabled() {
		return nil, nil
	}

	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var snapshots []Snapshot
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dir := filepath.Join(s.Dir, entry.Name())
		snapshot, err := readMeta(dir)
		if err != nil {
			continue
		}

		snapshot.Dir = dir
		snapshots = append(snapshots, snapshot)
	}

	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Created.After(snapshots[j].Created)
	})

	return snapshots, nil
}

func (s Store) Latest() (Snapshot, error) {
	snapshots, err := s.List()
	if err != nil {
		return Snapshot{}, err
	}
	if len(snapshots) == 0 {
		return Snapshot{}, ErrNoSnapshots
	}

	return snapshots[0], nil
}

func (s Store) Get(id string) (Snapshot, error) {
	if id == "" || id == "latest" {
		return s.Latest()
	}

	snapshots, err := s.List()
	if err != nil {
		return Snapshot{}, err
	}

	var match *Snapshot

	for i := range snapshots {
		snapshot := snapshots[i]
		if snapshot.ID == id {
			return snapshot, nil
		}
		if strings.HasPrefix(snapshot.ID, id) {
			if match != nil {
				return Snapshot{}, fmt.Errorf("%w: %s matches more than one backup", ErrNoSnapshots, id)
			}
			match = &snapshot
		}
	}

	if match != nil {
		return *match, nil
	}

	return Snapshot{}, fmt.Errorf("%w: %s", ErrNoSnapshots, id)
}

func (s Store) Restore(snapshot Snapshot) error {
	source := filepath.Join(snapshot.Dir, "cdm")
	if !cdm.Valid(source) {
		return fmt.Errorf("%w: %s is incomplete", ErrNoSnapshots, snapshot.ID)
	}

	if err := validTarget(snapshot.Target); err != nil {
		return err
	}

	return cdm.Copy(source, snapshot.Target)
}

func validTarget(target string) error {
	if !filepath.IsAbs(target) {
		return fmt.Errorf("%w: %q is not an absolute path", ErrBadTarget, target)
	}

	for _, part := range strings.Split(filepath.Clean(target), string(filepath.Separator)) {
		for _, name := range cdm.DirNames() {
			if part == name {
				return nil
			}
		}
	}

	return fmt.Errorf("%w: %q", ErrBadTarget, target)
}

func (s Store) Clear() (int, int64, error) {
	if !s.Enabled() {
		return 0, 0, nil
	}

	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, err
	}

	var count int
	var bytes int64

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dir := filepath.Join(s.Dir, entry.Name())
		_, size := measure(dir)

		if err := os.RemoveAll(dir); err != nil {
			return count, bytes, err
		}

		count++
		bytes += size
	}

	return count, bytes, nil
}

func (s Store) Prune() {
	if !s.Enabled() || s.Keep <= 0 {
		return
	}

	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return
	}

	dirs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}

	sort.Sort(sort.Reverse(sort.StringSlice(dirs)))

	for i := s.Keep; i < len(dirs); i++ {
		os.RemoveAll(filepath.Join(s.Dir, dirs[i]))
	}
}

func writeMeta(dir string, snapshot Snapshot) error {
	payload, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, metaFile), append(payload, '\n'), 0o644)
}

func readMeta(dir string) (Snapshot, error) {
	raw, err := os.ReadFile(filepath.Join(dir, metaFile))
	if err != nil {
		return Snapshot{}, err
	}

	var snapshot Snapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return Snapshot{}, err
	}

	return snapshot, nil
}

func measure(dir string) (files int, bytes int64) {
	filepath.WalkDir(dir, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return nil
		}

		files++
		bytes += info.Size()

		return nil
	})

	return files, bytes
}
