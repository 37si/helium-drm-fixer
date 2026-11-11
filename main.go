package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"charm.land/log/v2"

	"hdf/internal/backup"
	"hdf/internal/browser"
	conf "hdf/internal/config"
	"hdf/internal/fixer"
	"hdf/internal/pathutil"
	"hdf/internal/state"
	"hdf/internal/term"
	"hdf/internal/tui"
)

var version = "1.0.0"

var repo = "37si/helium-drm-fixer"

type config struct {
	browser      string
	source       string
	helium       string
	logPath      string
	repo         string
	rollback     string
	completion   string
	terminal     string
	configFile   string
	gmpVersion   string
	keep         int
	interval     time.Duration
	list         bool
	check        bool
	dryRun       bool
	yes          bool
	fresh        bool
	noAnim       bool
	noMouse      bool
	newest       bool
	noBackup     bool
	status       bool
	doctor       bool
	history      bool
	backups      bool
	clearBackups bool
	watch        bool
	verify       bool
	upgrade      bool
	json         bool
	online       bool
	external     bool
	noExternal   bool
	verbose      bool
	quiet        bool
	printConfig  bool
	showVersion  bool
}

type invocation struct {
	cfg      config
	settings conf.Settings
	file     string
	set      map[string]bool
}

func main() {
	if err := run(); err != nil {
		if errors.Is(err, errChecksFailed) {
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	migrateDirs()

	parsed, err := parseFlags(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	cfg := parsed.cfg

	if cfg.showVersion {
		fmt.Println("hdf", version)
		return nil
	}

	if cfg.completion != "" {
		return printCompletion(cfg.completion)
	}

	if cfg.printConfig {
		return printConfig(parsed)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger, closer, err := newLogger(cfg)
	if err != nil {
		return err
	}
	defer closer()

	opts, err := fixerOptions(cfg)
	if err != nil {
		return err
	}

	if cfg.clearBackups && (cfg.backups || cfg.rollback != "") {
		return errors.New("--clear-backups cannot be combined with --backups or --rollback")
	}

	switch {
	case cfg.upgrade:
		return upgrade(ctx, cfg, logger)
	case cfg.history:
		return printHistory(cfg, opts)
	case cfg.backups:
		return printBackups(cfg, opts)
	case cfg.clearBackups:
		return clearBackups(cfg, opts)
	case cfg.rollback != "":
		return rollback(ctx, cfg, opts, logger)
	case cfg.status:
		return printStatus(ctx, cfg, opts)
	case cfg.doctor:
		return printDoctor(ctx, cfg, opts)
	case cfg.verify:
		return openStream(cfg)
	case cfg.watch:
		return watchLoop(ctx, cfg, opts, logger)
	case cfg.list:
		return listSources(ctx, cfg, logger)
	case cfg.check:
		return check(ctx, cfg, opts, logger)
	case cfg.yes:
		return apply(ctx, cfg, opts, logger)
	}

	if !isTerminal() {
		return errors.New("stdout is not a terminal; pass --yes for headless mode, or --status / --doctor / --list / --check")
	}

	if handoff(cfg) {
		return nil
	}

	return tui.Run(ctx, tui.Options{
		Fixer:    opts,
		DryRun:   cfg.dryRun,
		LogPath:  cfg.logPath,
		Version:  version,
		Repo:     repository(cfg),
		Online:   cfg.online,
		NoAnim:   cfg.noAnim,
		NoMouse:  cfg.noMouse,
		Interval: cfg.interval,
	}, logger)
}

func parseFlags(args []string) (invocation, error) {
	parsed := invocation{set: make(map[string]bool)}

	flags := newFlagSet(&parsed.cfg)
	flags.SetOutput(io.Discard)

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Print(usageText())
			return parsed, flag.ErrHelp
		}
		return parsed, fmt.Errorf("%w, see --help", err)
	}

	flags.Visit(func(f *flag.Flag) { parsed.set[f.Name] = true })

	parsed.file = parsed.cfg.configFile
	if parsed.file == "" {
		parsed.file = conf.Path(dataDir())
	}

	if parsed.cfg.showVersion || parsed.cfg.completion != "" {
		return parsed, nil
	}

	settings, err := conf.Load(parsed.file)
	if err != nil {
		return parsed, err
	}

	parsed.settings = settings
	applySettings(&parsed)

	if parsed.cfg.keep < 0 {
		return parsed, errors.New("keep cannot be negative")
	}
	if parsed.cfg.logPath == "" {
		parsed.cfg.logPath = defaultLogPath()
	}

	return parsed, nil
}

func newFlagSet(cfg *config) *flag.FlagSet {
	flags := flag.NewFlagSet("hdf", flag.ContinueOnError)

	flags.StringVar(&cfg.browser, "browser", "", "only use this browser as the WidevineCdm source")
	flags.StringVar(&cfg.browser, "b", "", "shorthand for --browser")
	flags.StringVar(&cfg.source, "source-path", "", "WidevineCdm directory, install directory or browser binary")
	flags.StringVar(&cfg.helium, "helium", "", "Helium install directory")
	flags.StringVar(&cfg.logPath, "log-file", "", "write logs to this file")
	flags.StringVar(&cfg.repo, "repo", "", "GitHub repository for update checks, as owner/name")
	flags.StringVar(&cfg.rollback, "rollback", "", "restore a backup: an id, or \"latest\"")
	flags.StringVar(&cfg.completion, "completion", "", "print a completion script: bash, zsh, fish or powershell")
	flags.StringVar(&cfg.terminal, "terminal", "", "terminal emulator to open: an app name on macOS, a binary elsewhere")
	flags.StringVar(&cfg.configFile, "config", "", "settings file")
	flags.StringVar(&cfg.gmpVersion, "gmp-version", "", "Firefox version to ask Mozilla for the newest WidevineCdm")
	flags.IntVar(&cfg.keep, "keep", backup.DefaultKeep, "how many backups to keep, 0 disables them")
	flags.DurationVar(&cfg.interval, "interval", 30*time.Second, "poll interval for --watch")
	flags.BoolVar(&cfg.list, "list", false, "list detected browsers and exit")
	flags.BoolVar(&cfg.list, "l", false, "shorthand for --list")
	flags.BoolVar(&cfg.check, "check", false, "resolve source and target, then exit")
	flags.BoolVar(&cfg.status, "status", false, "show what Helium has and whether a newer CDM exists")
	flags.BoolVar(&cfg.status, "s", false, "shorthand for --status")
	flags.BoolVar(&cfg.doctor, "doctor", false, "run health checks and explain what to fix")
	flags.BoolVar(&cfg.doctor, "d", false, "shorthand for --doctor")
	flags.BoolVar(&cfg.history, "history", false, "list previous installs")
	flags.BoolVar(&cfg.backups, "backups", false, "list saved copies of replaced directories")
	flags.BoolVar(&cfg.clearBackups, "clear-backups", false, "delete every saved backup")
	flags.BoolVar(&cfg.watch, "watch", false, "reinstall automatically when Helium updates")
	flags.BoolVar(&cfg.verify, "verify", false, "open a Widevine test stream in Helium")
	flags.BoolVar(&cfg.upgrade, "upgrade", false, "install the newest hdf release")
	flags.BoolVar(&cfg.json, "json", false, "machine readable output")
	flags.BoolVar(&cfg.online, "online", false, "let --doctor check for a newer release")
	flags.BoolVar(&cfg.newest, "newest", false, "prefer the highest CDM version over browser order")
	flags.BoolVar(&cfg.fresh, "fresh", false, "download the newest WidevineCdm from Mozilla instead of copying one")
	flags.BoolVar(&cfg.noAnim, "no-anim", false, "turn off interface animations")
	flags.BoolVar(&cfg.noMouse, "no-mouse", false, "ignore the mouse")
	flags.BoolVar(&cfg.noBackup, "no-backup", false, "do not keep a copy of the directory being replaced")
	flags.BoolVar(&cfg.dryRun, "dry-run", false, "report without writing anything")
	flags.BoolVar(&cfg.dryRun, "n", false, "shorthand for --dry-run")
	flags.BoolVar(&cfg.yes, "yes", false, "run without the interface")
	flags.BoolVar(&cfg.yes, "y", false, "shorthand for --yes")
	flags.BoolVar(&cfg.external, "external", false, "always open the interface in its own window")
	flags.BoolVar(&cfg.noExternal, "no-external", false, "never leave the current terminal")
	flags.BoolVar(&cfg.verbose, "verbose", false, "log every lookup")
	flags.BoolVar(&cfg.verbose, "v", false, "shorthand for --verbose")
	flags.BoolVar(&cfg.quiet, "quiet", false, "no progress output")
	flags.BoolVar(&cfg.quiet, "q", false, "shorthand for --quiet")
	flags.BoolVar(&cfg.printConfig, "print-config", false, "show the settings in effect and exit")
	flags.BoolVar(&cfg.showVersion, "version", false, "print the version and exit")

	flags.Usage = func() {}

	return flags
}

func applySettings(parsed *invocation) {
	cfg, s, set := &parsed.cfg, parsed.settings, parsed.set

	if !set["browser"] && !set["b"] && s.Browser != "" {
		cfg.browser = s.Browser
	}
	if !set["helium"] && s.Helium != "" {
		cfg.helium = s.Helium
	}
	if !set["repo"] && s.Repo != "" {
		cfg.repo = s.Repo
	}
	if !set["keep"] && s.Keep != nil {
		cfg.keep = *s.Keep
	}
	if !set["interval"] {
		if value, ok := s.PollInterval(); ok {
			cfg.interval = value
		}
	}
	if !set["newest"] && s.Newest != nil {
		cfg.newest = *s.Newest
	}
	if !set["fresh"] && s.Fresh != nil {
		cfg.fresh = *s.Fresh
	}
	if !set["no-backup"] && s.NoBackup != nil {
		cfg.noBackup = *s.NoBackup
	}
	if !set["external"] && !set["no-external"] && s.External != nil {
		cfg.external = *s.External
	}
}

func dataDir() string { return state.DefaultDir() }

func cacheDir() string {
	base, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "hdf")
}

func migrateDirs() {
	pathutil.MigrateDir(state.DefaultDir())
	pathutil.MigrateDir(cacheDir())
	term.CleanupScripts()
}

func fixerOptions(cfg config) (fixer.Options, error) {
	opts := fixer.Options{
		SourcePath: cfg.source,
		HeliumPath: cfg.helium,
		Newest:     cfg.newest,
		Fresh:      cfg.fresh,
		GMPVersion: cfg.gmpVersion,
		CacheDir:   cacheDir(),
		Backup:     !cfg.noBackup && cfg.keep > 0,
		BackupDir:  filepath.Join(dataDir(), "backups"),
		Backups:    cfg.keep,
		StateDir:   dataDir(),
		Tool:       "hdf " + version,
	}

	if cfg.browser == "" {
		return opts, nil
	}

	id, ok := browser.Parse(cfg.browser)
	if !ok {
		return opts, fmt.Errorf("%w: %s", fixer.ErrUnknownBrowser, cfg.browser)
	}

	opts.Browser = id

	return opts, nil
}

func defaultLogPath() string {
	base, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "hdf", "hdf.log")
}

func newLogger(cfg config) (*log.Logger, func(), error) {
	closer := func() {}

	if cfg.logPath == "" {
		logger := log.New(os.Stderr)
		logger.SetLevel(logLevel(cfg))
		return logger, closer, nil
	}

	if err := os.MkdirAll(filepath.Dir(cfg.logPath), 0o755); err != nil {
		return nil, closer, fmt.Errorf("create log dir: %w", err)
	}

	rotateLog(cfg.logPath)

	file, err := os.OpenFile(cfg.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, closer, fmt.Errorf("open log file: %w", err)
	}

	var out io.Writer = file
	if cfg.verbose {
		out = io.MultiWriter(file, os.Stderr)
	}

	logger := log.New(out)
	logger.SetReportTimestamp(true)
	logger.SetLevel(logLevel(cfg))

	return logger, func() { file.Close() }, nil
}

func logLevel(cfg config) log.Level {
	if cfg.verbose {
		return log.DebugLevel
	}

	return log.InfoLevel
}

const maxLogSize = 1 << 20

func rotateLog(path string) {
	info, err := os.Stat(path)
	if err != nil || info.Size() < maxLogSize {
		return
	}

	os.Rename(path, path+".1")
}

func repository(cfg config) string {
	switch {
	case cfg.repo != "":
		return cfg.repo
	case os.Getenv("HDF_REPO") != "":
		return os.Getenv("HDF_REPO")
	}

	return repo
}

func sourceLine(source fixer.Source) string {
	parts := []string{source.Name}
	if source.Version != "" && !strings.Contains(source.Name, source.Version) {
		parts = append(parts, "CDM "+source.Version)
	}
	parts = append(parts, source.Path)
	return strings.Join(parts, " · ")
}

func isTerminal() bool {
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func handoff(cfg config) bool {
	if !wantsWindow(cfg) {
		return false
	}

	exe, err := os.Executable()
	if err != nil {
		return false
	}

	if temporaryBuild(exe) {
		return false
	}

	dir, err := os.Getwd()
	if err != nil {
		dir = ""
	}

	argv := append([]string{exe}, os.Args[1:]...)
	argv = append(argv, "--no-external")

	if err := term.Open(argv, term.Options{Terminal: cfg.terminal, Dir: dir}); err != nil {
		fmt.Fprintf(os.Stderr, "cannot open a separate window: %v\n", err)
		return false
	}

	fmt.Println("opened the interface in a new window")

	return true
}

func wantsWindow(cfg config) bool {
	switch {
	case os.Getenv(term.LaunchedEnv) != "":
		return false
	case cfg.noExternal:
		return false
	case cfg.external:
		return true
	}

	return term.Detect().Embedded()
}

func temporaryBuild(exe string) bool {
	for _, part := range strings.Split(filepath.ToSlash(strings.ToLower(exe)), "/") {
		if strings.HasPrefix(part, "go-build") {
			return true
		}
	}

	path := exe
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		path = resolved
	}

	for _, root := range []string{os.TempDir(), resolvedTemp()} {
		if root == "" {
			continue
		}

		if rel, err := filepath.Rel(root, path); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}

	return false
}

func resolvedTemp() string {
	resolved, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		return ""
	}

	return resolved
}
