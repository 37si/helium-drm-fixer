package status

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"hdf/internal/backup"
	"hdf/internal/browser"
	"hdf/internal/cdm"
	"hdf/internal/helium"
	"hdf/internal/pathutil"
	"hdf/internal/proc"
	"hdf/internal/state"
)

type Candidate struct {
	Available bool
	Name      string
	ID        browser.ID
	Version   string
	Path      string
}

type Installed struct {
	Path     string
	Version  string
	Managed  bool
	Source   string
	At       time.Time
	Size     int64
	Files    int
	Verified bool
}

type Report struct {
	Helium  helium.Install
	Found   bool
	CDM     *Installed
	Newest  Candidate
	Running proc.Status
	Backups int
	Checked time.Time
}

type Options struct {
	HeliumPath string
	StateDir   string
	BackupDir  string
	Backups    int
}

func (r Report) Outdated() bool {
	if r.CDM == nil || !r.Newest.Available || r.Newest.Version == "" {
		return false
	}

	return pathutil.CompareVersions(r.Newest.Version, r.CDM.Version) > 0
}

func Inspect(ctx context.Context, opts Options, detections []browser.Detection) Report {
	report := Report{Checked: time.Now()}

	report.Running = proc.Current(0).Helium("")

	install, err := helium.Find(opts.HeliumPath)
	if err != nil {
		return report
	}

	report.Helium = install
	report.Found = true

	if install.Root != "" {
		report.Running = proc.Current(proc.TTL).Helium(install.Root)
	}

	if cdm := installed(install, opts); cdm != nil {
		report.CDM = cdm
	}

	report.Newest = newest(ctx, detections)
	report.Backups = countBackups(opts)

	return report
}

func installed(install helium.Install, opts Options) *Installed {
	found := helium.Installed(install)
	if len(found) == 0 {
		return nil
	}

	newest := found[0]
	info := &Installed{Path: newest.Path, Version: newest.Version}
	info.Files, info.Size = measure(newest.Path)

	if marker, ok := state.ReadMarker(newest.Path); ok {
		info.Managed = true
		info.Source = marker.Source
		info.At = marker.InstalledAt
		info.Verified = marker.Verified
		return info
	}

	store := state.Open(opts.StateDir)
	if record, ok := store.Latest(newest.Path); ok {
		info.Managed = true
		info.Source = record.Source
		info.At = record.InstalledAt
		info.Verified = record.Verified
		return info
	}

	if stat, err := os.Stat(newest.Path); err == nil {
		info.At = stat.ModTime()
	}

	return info
}

func newest(ctx context.Context, detections []browser.Detection) Candidate {
	if detections == nil {
		detections = browser.Detect(ctx)
	}

	var best Candidate
	for _, detection := range detections {
		if !detection.Available() || detection.CDM.Version == "" {
			continue
		}

		if best.Available && pathutil.CompareVersions(detection.CDM.Version, best.Version) <= 0 {
			continue
		}

		best = Candidate{
			Available: true,
			Name:      detection.Name,
			ID:        detection.ID,
			Version:   detection.CDM.Version,
			Path:      detection.CDM.Path,
		}
	}

	return best
}

func countBackups(opts Options) int {
	snapshots, err := backup.New(opts.BackupDir, opts.Backups).List()
	if err != nil {
		return 0
	}

	return len(snapshots)
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

func Verify(dir string) bool { return cdm.Valid(dir) }
