package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/tabwriter"
	"time"

	"charm.land/log/v2"

	"hdf/internal/backup"
	"hdf/internal/browser"
	"hdf/internal/doctor"
	"hdf/internal/fixer"
	"hdf/internal/helium"
	"hdf/internal/state"
	"hdf/internal/status"
	"hdf/internal/units"
	"hdf/internal/update"
	"hdf/internal/verify"
	"hdf/internal/watch"
)

var errChecksFailed = errors.New("checks failed")

func listSources(ctx context.Context, cfg config, logger *log.Logger) error {
	detections := browser.Detect(ctx)
	if len(detections) == 0 {
		return fmt.Errorf("%w: no Chromium-based browser detected", fixer.ErrNoSource)
	}

	for _, detection := range detections {
		cdmPath := ""
		if detection.CDM != nil {
			cdmPath = detection.CDM.Path
		}
		logger.Debug("browser detected", "id", detection.ID, "binary", detection.Executable, "cdm", cdmPath)
	}

	if cfg.json {
		type row struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Version string `json:"version,omitempty"`
			Path    string `json:"path,omitempty"`
			Binary  string `json:"binary,omitempty"`
			Origin  string `json:"origin,omitempty"`
			Usable  bool   `json:"usable"`
		}

		rows := make([]row, 0, len(detections))
		for _, detection := range detections {
			entry := row{ID: string(detection.ID), Name: detection.Name, Binary: detection.Executable}
			if detection.CDM != nil {
				entry.Usable = true
				entry.Version = detection.CDM.Version
				entry.Path = detection.CDM.Path
				entry.Origin = string(detection.CDM.Origin)
			}
			rows = append(rows, entry)
		}

		return writeJSON(rows)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "BROWSER\tWIDEVINECDm\tVERSION\tLOCATION")

	for _, detection := range detections {
		state, version, location := "missing", "-", detection.Executable

		if detection.Available() {
			state = "available"
			location = detection.CDM.Path
			if detection.CDM.Version != "" {
				version = detection.CDM.Version
			}
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", detection.Name, state, version, location)
	}

	return w.Flush()
}

func check(ctx context.Context, cfg config, opts fixer.Options, logger *log.Logger) error {
	plan, err := fixer.Resolve(ctx, opts, logger, nil)
	if err != nil {
		return err
	}

	if cfg.json {
		return writeJSON(map[string]string{
			"source": sourceLine(plan.Source),
			"target": plan.Target,
			"hint":   plan.HintFile,
			"helium": plan.Helium.Root,
		})
	}

	fmt.Printf("source  %s\n", sourceLine(plan.Source))
	fmt.Printf("target  %s\n", plan.Target)
	if plan.HintFile != "" {
		fmt.Printf("hint    %s\n", plan.HintFile)
	}

	return nil
}

func apply(ctx context.Context, cfg config, opts fixer.Options, logger *log.Logger) error {
	plan, err := fixer.Resolve(ctx, opts, logger, nil)
	if err != nil {
		return err
	}

	if cfg.dryRun {
		fmt.Printf("source  %s\n", sourceLine(plan.Source))
		fmt.Printf("target  %s\n", plan.Target)
		if plan.HintFile != "" {
			fmt.Printf("hint    %s\n", plan.HintFile)
		}
		fmt.Println("dry run, nothing written")
		return nil
	}

	progress := isTerminal() && !cfg.quiet && !cfg.json

	result, err := fixer.Apply(ctx, plan, logger, func(step string, fraction float64) {
		if progress {
			fmt.Printf("\r%-56s %3.0f%%", step, fraction*100)
		}
	})
	if progress {
		fmt.Println()
	}

	if err != nil {
		return err
	}

	for _, warning := range result.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
	}

	if cfg.json {
		return writeJSON(map[string]string{
			"target":  result.Target,
			"version": result.Version,
			"source":  result.Source,
			"backup":  result.Backup,
		})
	}

	fmt.Printf("installed %s\n", result.Target)
	if result.Backup != "" {
		fmt.Printf("backup    %s, undo with --rollback latest\n", result.Backup)
	}
	fmt.Println("restart Helium for the change to take effect")

	return nil
}

type jsonCandidate struct {
	Browser string `json:"browser"`
	ID      string `json:"id"`
	Version string `json:"version"`
}

type jsonStatus struct {
	Found     bool           `json:"found"`
	Helium    string         `json:"helium,omitempty"`
	Location  string         `json:"location,omitempty"`
	Target    string         `json:"target,omitempty"`
	Version   string         `json:"version,omitempty"`
	Managed   bool           `json:"managed"`
	Source    string         `json:"source,omitempty"`
	Installed time.Time      `json:"installed_at,omitempty"`
	Bytes     int64          `json:"bytes,omitempty"`
	Files     int            `json:"files,omitempty"`
	Verified  bool           `json:"verified"`
	Running   bool           `json:"running"`
	RunningAt string         `json:"running_path,omitempty"`
	Newest    *jsonCandidate `json:"newest,omitempty"`
	Outdated  bool           `json:"outdated"`
	Backups   int            `json:"backups"`
	Checked   time.Time      `json:"checked"`
}

func samePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}

	return a == b
}

func runningLine(report status.Report) string {
	if !report.Running.Running {
		return "no"
	}

	line := "yes"
	if len(report.Running.PIDs) == 1 {
		line += fmt.Sprintf(" (pid %d)", report.Running.PIDs[0])
	}
	if report.Running.Matches {
		line += ", restart Helium to load a new CDM"
	}

	return line
}

func printStatus(ctx context.Context, cfg config, opts fixer.Options) error {
	report := status.Inspect(ctx, statusOptions(opts), nil)

	if cfg.json {
		out := jsonStatus{
			Found:    report.Found,
			Helium:   report.Helium.Root,
			Outdated: report.Outdated(),
			Backups:  report.Backups,
			Checked:  report.Checked,
			Running:  report.Running.Running,
		}

		if paths := report.Running.Paths; len(paths) > 0 {
			out.RunningAt = paths[0]
		}

		if report.CDM != nil {
			out.Location = report.CDM.Path
			if target, _, err := helium.Target(report.Helium, report.CDM.Version); err == nil {
				out.Target = target
			}
			out.Version = report.CDM.Version
			out.Managed = report.CDM.Managed
			out.Source = report.CDM.Source
			out.Installed = report.CDM.At
			out.Bytes = report.CDM.Size
			out.Files = report.CDM.Files
			out.Verified = report.CDM.Verified
		}

		if report.Newest.Available {
			out.Newest = &jsonCandidate{
				Browser: report.Newest.Name,
				ID:      string(report.Newest.ID),
				Version: report.Newest.Version,
			}
		}

		return writeJSON(out)
	}

	if !report.Found {
		fmt.Println("helium     not installed, or in a location this tool does not know")

		if report.Running.Running {
			fmt.Printf("running    yes, from %s\n", report.Running.Describe())
			fmt.Println()
			fmt.Println("pass --helium with that directory")
		}

		return nil
	}

	fmt.Printf("helium     %s\n", report.Helium.Root)

	if report.CDM == nil {
		fmt.Println("installed  nothing yet")
		fmt.Println()
		fmt.Println("run hdf to copy a CDM in from a browser")
		return nil
	}

	origin := "installed by something else"
	if report.CDM.Managed {
		origin = "installed by hdf"
		if report.CDM.Source != "" {
			origin += " from " + report.CDM.Source
		}
	}

	fmt.Printf("installed  %s · %s\n", versionOrUnknown(report.CDM.Version), origin)
	fmt.Printf("location   %s\n", report.CDM.Path)

	if target, _, err := helium.Target(report.Helium, report.CDM.Version); err == nil && !samePath(target, report.CDM.Path) {
		fmt.Printf("target     %s\n", target)
	}
	fmt.Printf("when       %s\n", units.Since(report.CDM.At))
	fmt.Printf("size       %s in %d files\n", units.Bytes(report.CDM.Size), report.CDM.Files)
	fmt.Printf("backups    %d saved\n", report.Backups)
	fmt.Printf("running    %s\n", runningLine(report))

	if report.Newest.Available {
		fmt.Printf("newest     %s (%s)\n", report.Newest.Version, report.Newest.Name)
	}

	fmt.Println()

	switch {
	case report.Outdated():
		fmt.Printf("update available: %s from %s\n", report.Newest.Version, report.Newest.Name)
	case report.Newest.Available:
		fmt.Println("up to date with the newest browser copy")
	default:
		fmt.Println("no browser can supply a CDM right now")
	}

	return nil
}

func printDoctor(ctx context.Context, cfg config, opts fixer.Options) error {
	checks := doctor.Run(ctx, doctorOptions(cfg, opts), nil)

	if cfg.json {
		type row struct {
			Name   string `json:"name"`
			Level  string `json:"level"`
			Detail string `json:"detail"`
			Hint   string `json:"hint,omitempty"`
		}

		rows := make([]row, 0, len(checks))
		for _, check := range checks {
			rows = append(rows, row{Name: check.Name, Level: check.Level.String(), Detail: check.Detail, Hint: check.Hint})
		}

		return writeJSON(rows)
	}

	for _, check := range checks {
		fmt.Printf("%s %-14s %s\n", symbol(check.Level), check.Name, check.Detail)
		if check.Hint != "" {
			fmt.Printf("  %-14s %s\n", "", check.Hint)
		}
	}

	fails, warns := doctor.Summary(checks)
	fmt.Printf("\n%d checks, %d failed, %d to look at\n", len(checks), fails, warns)

	if fails > 0 {
		return errChecksFailed
	}

	return nil
}

func printHistory(cfg config, opts fixer.Options) error {
	records, err := state.Open(opts.StateDir).Load()
	if err != nil {
		return err
	}

	if cfg.json {
		return writeJSON(records)
	}

	if len(records) == 0 {
		fmt.Println("nothing installed yet")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "WHEN\tVERSION\tSOURCE\tHOW\tTARGET")

	for i := len(records) - 1; i >= 0; i-- {
		record := records[i]
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			units.Since(record.InstalledAt),
			versionOrUnknown(record.Version),
			orDash(record.Source),
			orDash(record.Method),
			record.Target,
		)
	}

	return w.Flush()
}

func printBackups(cfg config, opts fixer.Options) error {
	snapshots, err := backup.New(opts.BackupDir, opts.Backups).List()
	if err != nil {
		return err
	}

	if cfg.json {
		return writeJSON(snapshots)
	}

	if len(snapshots) == 0 {
		fmt.Println("no backups yet, the first install keeps one")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tVERSION\tCREATED\tSIZE\tTARGET")

	var total int64
	for _, snapshot := range snapshots {
		total += snapshot.Bytes

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			snapshot.ID,
			versionOrUnknown(snapshot.Version),
			units.Since(snapshot.Created),
			units.Bytes(snapshot.Bytes),
			snapshot.Target,
		)
	}

	fmt.Fprintf(w, "\t\t\t%s\t%d kept\n", units.Bytes(total), len(snapshots))

	return w.Flush()
}

func clearBackups(cfg config, opts fixer.Options) error {
	store := backup.New(opts.BackupDir, opts.Backups)

	snapshots, err := store.List()
	if err != nil {
		return err
	}

	var total int64
	for _, snapshot := range snapshots {
		total += snapshot.Bytes
	}

	if len(snapshots) == 0 {
		if cfg.json {
			return writeJSON(map[string]any{"cleared": 0, "bytes": 0, "dir": opts.BackupDir})
		}
		fmt.Println("no backups to clear")
		return nil
	}

	if cfg.dryRun {
		if cfg.json {
			return writeJSON(map[string]any{"would_clear": len(snapshots), "bytes": total, "dir": opts.BackupDir})
		}
		fmt.Printf("would remove %d backups (%s) from %s\ndry run, nothing deleted\n", len(snapshots), units.Bytes(total), opts.BackupDir)
		return nil
	}

	if !cfg.yes {
		if !inputIsTerminal() {
			return errors.New("clearing backups needs --yes or an interactive terminal")
		}

		fmt.Fprintf(os.Stderr, "%d backups use %s in %s\n", len(snapshots), units.Bytes(total), opts.BackupDir)
		reader := bufio.NewReader(os.Stdin)

		if !confirm(reader, "Delete all of them? [y/N] ") {
			fmt.Fprintln(os.Stderr, "cancelled")
			return nil
		}
		if !confirmWord(reader, "Type DELETE to confirm: ", "DELETE") {
			fmt.Fprintln(os.Stderr, "cancelled")
			return nil
		}
	}

	count, bytes, err := store.Clear()
	if err != nil {
		return err
	}

	if cfg.json {
		return writeJSON(map[string]any{"cleared": count, "bytes": bytes, "dir": opts.BackupDir})
	}

	fmt.Printf("removed %d backups (%s)\n", count, units.Bytes(bytes))
	return nil
}

func confirm(reader *bufio.Reader, prompt string) bool {
	answer := strings.ToLower(readAnswer(reader, prompt))

	return answer == "y" || answer == "yes"
}

func confirmWord(reader *bufio.Reader, prompt, want string) bool {
	return readAnswer(reader, prompt) == want
}

func readAnswer(reader *bufio.Reader, prompt string) string {
	fmt.Fprint(os.Stderr, prompt)

	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return ""
	}

	return strings.TrimSpace(line)
}

func inputIsTerminal() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}

	return info.Mode()&os.ModeCharDevice != 0
}

func rollback(_ context.Context, cfg config, opts fixer.Options, logger *log.Logger) error {
	store := backup.New(opts.BackupDir, opts.Backups)

	snapshot, err := store.Get(cfg.rollback)
	if err != nil {
		return err
	}

	if cfg.dryRun {
		fmt.Printf("would restore %s into %s\n", snapshot.ID, snapshot.Target)
		return nil
	}

	result, err := fixer.Rollback(snapshot, fixer.Plan{
		BackupDir: opts.BackupDir,
		Backups:   opts.Backups,
		StateDir:  opts.StateDir,
		Tool:      opts.Tool,
	}, logger, nil)
	if err != nil {
		return err
	}

	for _, warning := range result.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
	}

	fmt.Printf("restored %s into %s\n", snapshot.ID, result.Target)
	fmt.Println("restart Helium for the change to take effect")

	return nil
}

func watchLoop(ctx context.Context, cfg config, opts fixer.Options, logger *log.Logger) error {
	if cfg.browser == "" {
		opts.Newest = true
	}

	interval := cfg.interval
	if interval < watch.MinInterval {
		interval = watch.MinInterval
	}

	watcher := watch.New(watch.Options{Interval: interval, Fixer: opts, DryRun: cfg.dryRun}, logger)

	if !cfg.quiet {
		fmt.Printf("watching Helium every %s, ctrl-c to stop\n\n", units.Duration(interval))
	}

	for event := range watcher.Run(ctx) {
		fmt.Printf("%s  %s\n", event.At.Format("15:04:05"), event.Message)
	}

	return nil
}

func openStream(cfg config) error {
	install, err := helium.Find(cfg.helium)
	if err != nil {
		return err
	}

	stream := verify.Default()
	if err := verify.Open(install, stream); err != nil {
		return err
	}

	fmt.Printf("opened %s\n%s\n", stream.Name, stream.URL)
	fmt.Println("if the video plays, the CDM is working")

	return nil
}

func upgrade(ctx context.Context, cfg config, logger *log.Logger) error {
	target := repository(cfg)
	if target == "" {
		return errors.New("no repository configured; pass --repo owner/name or set HDF_REPO")
	}

	release, err := update.Latest(ctx, target)
	if err != nil {
		return err
	}

	if !update.Newer(release.Version(), version) {
		fmt.Printf("hdf %s is the newest release\n", version)
		return nil
	}

	fmt.Printf("hdf %s is available, this build is %s\n", release.Version(), version)

	if cfg.dryRun {
		fmt.Println("dry run, nothing installed")
		return nil
	}

	asset, ok := release.Binary()
	if !ok {
		return fmt.Errorf("release %s has no build for %s/%s", release.Tag, runtime.GOOS, runtime.GOARCH)
	}

	executable, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}

	staged := executable + ".new"
	if err := update.Download(ctx, asset, staged); err != nil {
		return err
	}
	defer os.Remove(staged)

	if sums, ok := release.Checksums(); ok {
		want, err := update.Sums(ctx, sums, asset.Name)
		if err != nil {
			return err
		}

		got, err := update.Digest(staged)
		if err != nil {
			return err
		}

		if !strings.EqualFold(want, got) {
			return fmt.Errorf("%w: %s", update.ErrChecksum, asset.Name)
		}

		fmt.Println("checksum matches sha256.txt")
	} else {
		fmt.Println("release has no sha256.txt, installing without a checksum")
	}

	if err := update.Replace(executable, staged); err != nil {
		return err
	}

	logger.Info("updated", "from", version, "to", release.Version())
	fmt.Printf("updated to %s\n", release.Version())

	return nil
}

func statusOptions(opts fixer.Options) status.Options {
	return status.Options{
		HeliumPath: opts.HeliumPath,
		StateDir:   opts.StateDir,
		BackupDir:  opts.BackupDir,
		Backups:    opts.Backups,
	}
}

func doctorOptions(cfg config, opts fixer.Options) doctor.Options {
	return doctor.Options{
		HeliumPath: opts.HeliumPath,
		StateDir:   opts.StateDir,
		BackupDir:  opts.BackupDir,
		Backups:    opts.Backups,
		LogPath:    cfg.logPath,
		Version:    version,
		Repo:       repository(cfg),
		Online:     cfg.online,
	}
}

func symbol(level doctor.Level) string {
	switch level {
	case doctor.LevelOK:
		return "✓"
	case doctor.LevelWarn:
		return "!"
	case doctor.LevelFail:
		return "✗"
	}

	return "·"
}

func writeJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")

	return encoder.Encode(value)
}

func versionOrUnknown(version string) string {
	if version == "" {
		return "unknown version"
	}

	return version
}

func orDash(value string) string {
	if value == "" {
		return "-"
	}

	return value
}
