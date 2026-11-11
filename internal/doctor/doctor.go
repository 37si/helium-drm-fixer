package doctor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"hdf/internal/backup"
	"hdf/internal/browser"
	"hdf/internal/disk"
	"hdf/internal/gmp"
	"hdf/internal/helium"
	"hdf/internal/pathutil"
	"hdf/internal/status"
	"hdf/internal/units"
	"hdf/internal/update"
)

type Level int

const (
	LevelOK Level = iota
	LevelWarn
	LevelFail
	LevelInfo
)

func (l Level) String() string {
	switch l {
	case LevelOK:
		return "ok"
	case LevelWarn:
		return "warn"
	case LevelFail:
		return "fail"
	}

	return "info"
}

type Check struct {
	Name   string
	Level  Level
	Detail string
	Hint   string
}

type Options struct {
	HeliumPath string
	StateDir   string
	BackupDir  string
	Backups    int
	LogPath    string
	Version    string
	Repo       string
	Online     bool
}

func Run(ctx context.Context, opts Options, detections []browser.Detection) []Check {
	if detections == nil {
		detections = browser.Detect(ctx)
	}

	report := status.Inspect(ctx, opts.statusOptions(), detections)

	checks := []Check{
		checkHelium(report),
		checkInstalled(report),
		checkProvenance(report),
		checkVersion(report),
		checkSources(detections),
		checkBackups(opts),
		checkWritable(report),
		checkRunning(report),
		checkSpace(report),
		checkLog(opts),
	}

	if opts.Online {
		checks = append(checks, checkFresh(ctx, report))
	}

	if opts.Online && opts.Repo != "" {
		checks = append(checks, checkUpdate(ctx, opts))
	}

	return checks
}

func Summary(checks []Check) (fails, warns int) {
	for _, check := range checks {
		switch check.Level {
		case LevelFail:
			fails++
		case LevelWarn:
			warns++
		}
	}

	return fails, warns
}

func (o Options) statusOptions() status.Options {
	return status.Options{
		HeliumPath: o.HeliumPath,
		StateDir:   o.StateDir,
		BackupDir:  o.BackupDir,
		Backups:    o.Backups,
	}
}

func checkHelium(report status.Report) Check {
	if !report.Found {
		return Check{
			Name:   "Helium",
			Level:  LevelFail,
			Detail: "not installed, or in a location this tool does not know",
			Hint:   "pass --helium with the install directory",
		}
	}

	return Check{
		Name:   "Helium",
		Level:  LevelOK,
		Detail: report.Helium.Root,
	}
}

func checkInstalled(report status.Report) Check {
	if !report.Found {
		return Check{Name: "WidevineCdm", Level: LevelInfo, Detail: "skipped, Helium was not found"}
	}

	if report.CDM == nil {
		return Check{
			Name:   "WidevineCdm",
			Level:  LevelFail,
			Detail: "not installed",
			Hint:   "run hdf to copy one in from a browser",
		}
	}

	return Check{
		Name:   "WidevineCdm",
		Level:  LevelOK,
		Detail: fmt.Sprintf("%s · %s · %d files", versionOrUnknown(report.CDM.Version), units.Bytes(report.CDM.Size), report.CDM.Files),
	}
}

func checkProvenance(report status.Report) Check {
	if report.CDM == nil {
		return Check{Name: "origin", Level: LevelInfo, Detail: "nothing installed yet"}
	}

	if !report.CDM.Managed {
		return Check{
			Name:   "origin",
			Level:  LevelWarn,
			Detail: "installed by something else, or before this tool kept records",
			Hint:   "run hdf to take ownership of the directory",
		}
	}

	detail := "installed by hdf"
	if report.CDM.Source != "" {
		detail += " from " + report.CDM.Source
	}
	if !report.CDM.At.IsZero() {
		detail += ", " + units.Since(report.CDM.At)
	}
	if report.CDM.Verified {
		detail += ", playback confirmed"
	}

	return Check{Name: "origin", Level: LevelOK, Detail: detail}
}

func checkVersion(report status.Report) Check {
	if report.CDM == nil {
		return Check{Name: "version", Level: LevelInfo, Detail: "nothing installed yet"}
	}

	if !report.Newest.Available {
		return Check{
			Name:   "version",
			Level:  LevelInfo,
			Detail: versionOrUnknown(report.CDM.Version) + ", no browser can supply a newer one",
		}
	}

	if report.Outdated() {
		return Check{
			Name:   "version",
			Level:  LevelWarn,
			Detail: fmt.Sprintf("%s installed, %s available from %s", report.CDM.Version, report.Newest.Version, report.Newest.Name),
			Hint:   "run hdf and pick " + report.Newest.Name,
		}
	}

	return Check{
		Name:   "version",
		Level:  LevelOK,
		Detail: fmt.Sprintf("%s, as new as the newest browser copy", report.CDM.Version),
	}
}

func checkSources(detections []browser.Detection) Check {
	installed, usable := 0, 0

	for _, detection := range detections {
		if detection.Executable == "" && detection.CDM == nil {
			continue
		}

		installed++
		if detection.Available() {
			usable++
		}
	}

	switch {
	case usable > 0:
		return Check{
			Name:   "sources",
			Level:  LevelOK,
			Detail: fmt.Sprintf("%d of %d browsers can supply a CDM", usable, installed),
		}
	case installed > 0:
		return Check{
			Name:   "sources",
			Level:  LevelWarn,
			Detail: fmt.Sprintf("%d browsers found, none ships a CDM", installed),
			Hint:   "open the browser once so its component updater fetches Widevine",
		}
	}

	return Check{
		Name:   "sources",
		Level:  LevelWarn,
		Detail: "no Chromium-based browser found",
		Hint:   "install Chrome, Edge, Brave or Vivaldi",
	}
}

func checkBackups(opts Options) Check {
	snapshots, err := backup.New(opts.BackupDir, opts.Backups).List()
	if err != nil {
		return Check{Name: "backups", Level: LevelWarn, Detail: err.Error()}
	}

	if len(snapshots) == 0 {
		return Check{
			Name:   "backups",
			Level:  LevelInfo,
			Detail: "none yet, the first install will keep a copy",
		}
	}

	newest := snapshots[0]

	return Check{
		Name:   "backups",
		Level:  LevelOK,
		Detail: fmt.Sprintf("%d kept, newest %s from %s", len(snapshots), versionOrUnknown(newest.Version), units.Since(newest.Created)),
	}
}

func checkWritable(report status.Report) Check {
	if !report.Found {
		return Check{Name: "write access", Level: LevelInfo, Detail: "skipped, Helium was not found"}
	}

	dir := writableDir(report)

	probe, err := os.CreateTemp(dir, ".hdf-probe")
	if err != nil {
		return Check{
			Name:   "write access",
			Level:  LevelFail,
			Detail: err.Error(),
			Hint:   "run as an administrator, or check the folder permissions",
		}
	}

	name := probe.Name()
	probe.Close()
	os.Remove(name)

	return Check{Name: "write access", Level: LevelOK, Detail: "the CDM directory is writable"}
}

func writableDir(report status.Report) string {
	dir, err := helium.Root(report.Helium)
	if err != nil || dir == "" {
		return report.Helium.Dir
	}

	for {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return report.Helium.Dir
		}

		dir = parent
	}
}

func checkRunning(report status.Report) Check {
	running := report.Running

	if !report.Found && !running.Running {
		return Check{Name: "running", Level: LevelInfo, Detail: "skipped, Helium was not found"}
	}

	if !running.Running {
		return Check{Name: "running", Level: LevelOK, Detail: "Helium is not running"}
	}

	detail := "Helium is running, so a new CDM will not load until it restarts"
	if len(running.Paths) > 0 {
		detail = fmt.Sprintf("Helium is running from %s, a restart is needed after installing", running.Paths[0])
	}

	return Check{
		Name:   "running",
		Level:  LevelWarn,
		Detail: detail,
		Hint:   "quit Helium and start it again after installing",
	}
}

func checkSpace(report status.Report) Check {
	if !report.Found {
		return Check{Name: "disk", Level: LevelInfo, Detail: "skipped, Helium was not found"}
	}

	free, err := disk.Free(writableDir(report))
	if err != nil {
		return Check{Name: "disk", Level: LevelInfo, Detail: "free space could not be read"}
	}

	if free < 250<<20 {
		return Check{
			Name:   "disk",
			Level:  LevelWarn,
			Detail: units.Bytes(free) + " free",
			Hint:   "a CDM needs about 20 MB",
		}
	}

	return Check{Name: "disk", Level: LevelOK, Detail: units.Bytes(free) + " free"}
}

func checkLog(opts Options) Check {
	if opts.LogPath == "" {
		return Check{Name: "log", Level: LevelInfo, Detail: "writing to stderr"}
	}

	dir := filepath.Dir(opts.LogPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Check{Name: "log", Level: LevelWarn, Detail: err.Error()}
	}

	file, err := os.OpenFile(opts.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return Check{Name: "log", Level: LevelWarn, Detail: err.Error()}
	}
	file.Close()

	return Check{Name: "log", Level: LevelOK, Detail: opts.LogPath}
}

func checkFresh(ctx context.Context, report status.Report) Check {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	client, err := gmp.New("", nil)
	if err != nil {
		return Check{Name: "newest", Level: LevelInfo, Detail: "could not reach Mozilla"}
	}

	release, err := client.Latest(ctx, "")
	if err != nil {
		return Check{
			Name:   "newest",
			Level:  LevelInfo,
			Detail: "Mozilla could not be asked",
			Hint:   "check the network, or pass --fresh to try a download",
		}
	}

	detail := "Mozilla serves " + release.Version + " to Firefox " + release.Firefox

	if report.CDM == nil || report.CDM.Version == "" {
		return Check{Name: "newest", Level: LevelInfo, Detail: detail, Hint: "run hdf --fresh"}
	}

	switch cmp := pathutil.CompareVersions(release.Version, report.CDM.Version); {
	case cmp > 0:
		return Check{
			Name:   "newest",
			Level:  LevelWarn,
			Detail: report.CDM.Version + " installed, " + release.Version + " is current",
			Hint:   "run hdf --fresh to download it",
		}
	case cmp == 0:
		return Check{Name: "newest", Level: LevelOK, Detail: report.CDM.Version + " is what Mozilla serves"}
	}

	return Check{Name: "newest", Level: LevelOK, Detail: report.CDM.Version + " is newer than " + release.Version}
}

func checkUpdate(ctx context.Context, opts Options) Check {
	release, err := update.Latest(ctx, opts.Repo)
	if err != nil {
		return Check{Name: "hdf", Level: LevelInfo, Detail: "could not check for a newer build"}
	}

	if update.Newer(release.Version(), opts.Version) {
		return Check{
			Name:   "hdf",
			Level:  LevelWarn,
			Detail: fmt.Sprintf("%s installed, %s released", opts.Version, release.Version()),
			Hint:   "run hdf --upgrade",
		}
	}

	return Check{Name: "hdf", Level: LevelOK, Detail: opts.Version + " is the newest release"}
}

func versionOrUnknown(version string) string {
	if version == "" {
		return "unknown version"
	}

	return version
}
