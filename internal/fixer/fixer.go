package fixer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"charm.land/log/v2"

	"hdf/internal/backup"
	"hdf/internal/browser"
	"hdf/internal/cdm"
	"hdf/internal/gmp"
	"hdf/internal/helium"
	"hdf/internal/pathutil"
	"hdf/internal/state"
)

var (
	ErrNoSource       = errors.New("no WidevineCdm source found")
	ErrUnknownBrowser = errors.New("unknown browser")
	ErrNoVersion      = errors.New("could not determine the WidevineCdm version")
	ErrSourceIsTarget = errors.New("the source is the directory being replaced")
)

type Options struct {
	Browser    browser.ID
	SourcePath string
	HeliumPath string
	Newest     bool
	Fresh      bool
	GMPVersion string
	CacheDir   string
	Backup     bool
	BackupDir  string
	Backups    int
	StateDir   string
	Tool       string
}

const (
	KindCopy     = "copy"
	KindDownload = "download"
)

type Source struct {
	BrowserID browser.ID
	Name      string
	Path      string
	Version   string
	Origin    string
	Kind      string
	Note      string
}

type Plan struct {
	Source      Source
	Helium      helium.Install
	Target      string
	HintFile    string
	NeedsResign bool
	Backup      bool
	BackupDir   string
	Backups     int
	StateDir    string
	Tool        string
}

type Reporter func(step string, fraction float64)

type Result struct {
	Target   string
	Version  string
	Source   string
	Backup   string
	Warnings []string
}

func Resolve(ctx context.Context, opts Options, logger *log.Logger, report Reporter) (Plan, error) {
	if report == nil {
		report = func(string, float64) {}
	}

	if opts.Browser == browser.Fresh {
		opts.Fresh = true
		opts.Browser = ""
	}

	report("scanning for browsers", 0.05)
	if logger != nil {
		logger.Debug("scanning for WidevineCdm", "browser", opts.Browser, "source", opts.SourcePath, "fresh", opts.Fresh)
	}

	var detections []browser.Detection
	switch {
	case opts.SourcePath != "":
	case opts.Browser != "":
		if detection, ok := browser.DetectOne(ctx, opts.Browser); ok {
			detections = []browser.Detection{detection}
		}
	default:
		detections = browser.Detect(ctx)
	}

	source, err := pickSource(ctx, opts, detections, logger, report)
	if err != nil {
		return Plan{}, err
	}

	report("locating Helium", 0.75)

	install, err := helium.Find(opts.HeliumPath)
	if err != nil {
		return Plan{}, err
	}

	version := source.Version
	if version == "" {
		version, err = cdm.Version(source.Path)
		if err != nil {
			return Plan{}, fmt.Errorf("%w: %s", ErrNoVersion, source.Path)
		}
		source.Version = version
	}

	target, hintFile, err := helium.Target(install, version)
	if err != nil {
		return Plan{}, err
	}

	if cdm.Overlaps(source.Path, target) {
		return Plan{}, fmt.Errorf("%w: %s", ErrSourceIsTarget, target)
	}

	if logger != nil {
		logger.Debug("resolved WidevineCdm plan", "source", source.Path, "version", version, "target", target, "helium", install.Root)
	}

	return Plan{
		Source:      source,
		Helium:      install,
		Target:      target,
		HintFile:    hintFile,
		NeedsResign: needsResign(),
		Backup:      opts.Backup,
		BackupDir:   opts.BackupDir,
		Backups:     opts.Backups,
		StateDir:    opts.StateDir,
		Tool:        opts.Tool,
	}, nil
}

func pickSource(ctx context.Context, opts Options, detections []browser.Detection, logger *log.Logger, report Reporter) (Source, error) {
	if report == nil {
		report = func(string, float64) {}
	}

	if opts.SourcePath != "" {
		path, err := resolveCustomPath(opts.SourcePath)
		if err != nil {
			return Source{}, err
		}

		source := Source{Name: "custom path", Path: path, Origin: "custom", Kind: KindCopy}
		if version, err := cdm.Version(path); err == nil {
			source.Version = version
		}
		return source, nil
	}

	var note string

	if opts.Fresh {
		source, err := freshSource(ctx, opts, logger, report)
		if err == nil {
			return source, nil
		}

		note = "fresh download failed, copied from a browser instead: " + err.Error()
		logger.Warn("could not download a fresh WidevineCdm", "err", err)
	}

	var candidates []Source

	for _, detection := range detections {
		if opts.Browser != "" && detection.ID != opts.Browser {
			continue
		}
		if !detection.Available() {
			continue
		}

		candidates = append(candidates, Source{
			BrowserID: detection.ID,
			Name:      detection.Name,
			Path:      detection.CDM.Path,
			Version:   detection.CDM.Version,
			Origin:    string(detection.CDM.Origin),
			Kind:      KindCopy,
		})
	}

	if len(candidates) > 0 {
		best := candidates[0]

		if opts.Newest {
			for _, candidate := range candidates[1:] {
				if pathutil.CompareVersions(candidate.Version, best.Version) > 0 {
					best = candidate
				}
			}
		}

		best.Note = note

		return best, nil
	}

	if opts.Browser != "" {
		name := string(opts.Browser)
		if b, ok := browser.Get(opts.Browser); ok {
			name = b.Name
		}
		return Source{}, fmt.Errorf("%w: %s is not installed or has no CDM", ErrNoSource, name)
	}

	if note != "" {
		return Source{}, fmt.Errorf("%w: %s", ErrNoSource, note)
	}

	return Source{}, fmt.Errorf("%w: install a Chromium browser or run with --fresh", ErrNoSource)
}

func freshSource(ctx context.Context, opts Options, logger *log.Logger, report Reporter) (Source, error) {
	client, err := gmp.New(opts.CacheDir, logger)
	if err != nil {
		return Source{}, err
	}

	report("asking Mozilla for the newest WidevineCdm", 0.1)

	release, err := client.Latest(ctx, opts.GMPVersion)
	if err != nil {
		return Source{}, err
	}

	report("downloading WidevineCdm "+release.Version, 0.2)

	path, err := client.Fetch(ctx, release, func(done, total int64) {
		if total <= 0 {
			return
		}

		share := float64(done) / float64(total)
		report(fmt.Sprintf("downloading WidevineCdm %s, %d%%", release.Version, int(share*100)), 0.2+0.5*share)
	})
	if err != nil {
		return Source{}, err
	}

	return Source{
		BrowserID: browser.Fresh,
		Name:      "Mozilla · WidevineCdm " + release.Version,
		Path:      path,
		Version:   release.Version,
		Origin:    "download",
		Kind:      KindDownload,
	}, nil
}

func resolveCustomPath(input string) (string, error) {
	expanded, ok := pathutil.Expand(input)
	if !ok {
		expanded = input
	}

	if abs, err := filepath.Abs(expanded); err == nil {
		expanded = abs
	}

	if cdm.Valid(expanded) {
		return expanded, nil
	}

	info, err := os.Stat(expanded)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrNoSource, input)
	}

	if location, ok := browser.Locate(expanded); ok {
		return location.Path, nil
	}

	if info.IsDir() {
		return "", fmt.Errorf("%w: no WidevineCdm in %s", ErrNoSource, expanded)
	}

	return "", fmt.Errorf("%w: no WidevineCdm near %s", ErrNoSource, expanded)
}

func Apply(ctx context.Context, plan Plan, logger *log.Logger, report Reporter) (Result, error) {
	if report == nil {
		report = func(string, float64) {}
	}

	result := Result{
		Target:  plan.Target,
		Version: plan.Source.Version,
		Source:  plan.Source.Name,
	}

	if plan.Source.Note != "" {
		result.Warnings = append(result.Warnings, plan.Source.Note)
	}

	if plan.Backup && plan.BackupDir != "" {
		report("saving a backup", 0.06)

		snapshot, err := backup.New(plan.BackupDir, plan.Backups).Create(superseded(plan))
		switch {
		case err != nil:
			result.Warnings = append(result.Warnings, fmt.Sprintf("backup: %v", err))
			logger.Warn("could not save a backup", "err", err)
		case snapshot.ID != "":
			result.Backup = snapshot.ID
		}
	}

	report("copying WidevineCdm", 0.1)

	if err := cdm.Copy(plan.Source.Path, plan.Target); err != nil {
		return Result{}, fmt.Errorf("copy WidevineCdm: %w", err)
	}

	if plan.HintFile != "" {
		report("writing component updater hint", 0.7)
		if err := helium.WriteHint(plan.HintFile, plan.Target); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("hint file: %v", err))
			logger.Warn("could not write hint file", "err", err)
		}
	}

	method := state.MethodCopy
	if plan.Source.Kind == KindDownload {
		method = state.MethodDownload
	}

	record(plan, result, method, logger)

	if plan.NeedsResign {
		report("re-signing Helium.app", 0.85)
		if err := resign(ctx, plan.Helium.Root, plan.Target); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("re-sign: %v", err))
			logger.Warn("could not re-sign Helium.app", "err", err)
		}
	}

	report("done", 1)

	return result, nil
}

func Rollback(snapshot backup.Snapshot, plan Plan, logger *log.Logger, report Reporter) (Result, error) {
	if report == nil {
		report = func(string, float64) {}
	}

	report("restoring the backup", 0.2)

	if err := backup.New(plan.BackupDir, plan.Backups).Restore(snapshot); err != nil {
		return Result{}, fmt.Errorf("restore %s: %w", snapshot.ID, err)
	}

	hint := filepath.Join(filepath.Dir(snapshot.Target), helium.HintFile)
	if _, err := os.Stat(hint); err == nil {
		if err := helium.WriteHint(hint, snapshot.Target); err != nil {
			logger.Warn("could not repoint the component updater hint", "err", err)
		}
	}

	version, _ := cdm.Version(snapshot.Target)

	result := Result{
		Target:  snapshot.Target,
		Version: version,
		Source:  "backup " + snapshot.ID,
		Backup:  snapshot.ID,
	}

	record(plan, result, state.MethodRollback, logger)

	if needsResign() {
		report("re-signing Helium.app", 0.9)
		if err := resign(context.Background(), plan.Helium.Root, snapshot.Target); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("re-sign: %v", err))
			logger.Warn("could not re-sign Helium.app", "err", err)
		}
	}

	report("done", 1)

	return result, nil
}

func superseded(plan Plan) string {
	if cdm.Valid(plan.Target) {
		return plan.Target
	}

	if found := helium.Installed(plan.Helium); len(found) > 0 {
		return found[0].Path
	}

	return plan.Target
}

func record(plan Plan, result Result, method string, logger *log.Logger) {
	entry := state.Record{
		Target:      result.Target,
		Version:     result.Version,
		Source:      result.Source,
		SourceID:    string(plan.Source.BrowserID),
		SourcePath:  plan.Source.Path,
		Tool:        plan.Tool,
		Method:      method,
		InstalledAt: time.Now(),
		Backup:      result.Backup,
	}

	if err := state.WriteMarker(result.Target, entry); err != nil {
		logger.Warn("could not write the install marker", "err", err)
	}

	if err := state.Open(plan.StateDir).Append(entry); err != nil {
		logger.Warn("could not append to the history", "err", err)
	}
}
