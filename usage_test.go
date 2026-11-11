package main

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUsageListsEveryFlag(t *testing.T) {
	var cfg config
	fs := newFlagSet(&cfg)

	help := usageText()
	fs.VisitAll(func(f *flag.Flag) {
		if !strings.Contains(help, "--"+f.Name) {
			t.Errorf("--%s is missing from the help", f.Name)
		}
	})
}

func TestUsageGroupsEveryFlag(t *testing.T) {
	listed := make(map[string]bool)
	for _, group := range usageGroups {
		for _, row := range group.rows {
			for _, name := range strings.Split(row.names, ",") {
				name = strings.TrimSpace(name)
				name = strings.TrimPrefix(strings.TrimPrefix(name, "-"), "-")
				listed[name] = true
			}
		}
	}

	var cfg config
	fs := newFlagSet(&cfg)
	fs.VisitAll(func(f *flag.Flag) {
		if !listed[f.Name] {
			t.Errorf("--%s is registered but not in a help group", f.Name)
		}
	})
}

func TestVersionAndCompletionIgnoreABrokenSettingsFile(t *testing.T) {
	path := writeSettings(t, "browser = netscape\n")

	for _, args := range [][]string{
		{"--config", path, "--version"},
		{"--config", path, "--completion", "bash"},
	} {
		if _, err := parseFlags(args); err != nil {
			t.Fatalf("parseFlags(%v) = %v", args, err)
		}
	}
}

func TestCompletionListsEveryLongFlag(t *testing.T) {
	var cfg config
	fs := newFlagSet(&cfg)

	bodies := map[string]string{
		"bash":       bashCompletion,
		"zsh":        zshCompletion,
		"fish":       fishCompletion,
		"powershell": powerShellCompletion,
	}

	fs.VisitAll(func(f *flag.Flag) {
		if len(f.Name) < 2 {
			return
		}

		for shell, body := range bodies {
			if strings.Contains(body, "--"+f.Name) {
				continue
			}
			if shell == "fish" && strings.Contains(body, "-l "+f.Name) {
				continue
			}
			t.Errorf("%s completion is missing --%s", shell, f.Name)
		}
	})
}

func TestCompletionOffersFresh(t *testing.T) {
	found := false

	for _, id := range browserIDs() {
		if id == "fresh" {
			found = true
		}
	}

	if !found {
		t.Fatal("the fresh source is missing from browser completion")
	}
}

func writeSettings(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.cfg")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	return path
}

func TestSettingsFillTheGaps(t *testing.T) {
	path := writeSettings(t, "browser = brave\nkeep = 2\ninterval = 90s\n")

	parsed, err := parseFlags([]string{"--config", path})
	if err != nil {
		t.Fatal(err)
	}

	if parsed.cfg.browser != "brave" {
		t.Fatalf("browser = %q", parsed.cfg.browser)
	}
	if parsed.cfg.keep != 2 {
		t.Fatalf("keep = %d", parsed.cfg.keep)
	}
	if parsed.cfg.interval != 90*time.Second {
		t.Fatalf("interval = %v", parsed.cfg.interval)
	}
	if got := parsed.origin("browser", "b", true); got != "settings" {
		t.Fatalf("origin = %q", got)
	}
}

func TestFlagsBeatSettings(t *testing.T) {
	path := writeSettings(t, "browser = brave\nkeep = 2\n")

	parsed, err := parseFlags([]string{"--config", path, "--browser", "chrome", "--keep", "7"})
	if err != nil {
		t.Fatal(err)
	}

	if parsed.cfg.browser != "chrome" || parsed.cfg.keep != 7 {
		t.Fatalf("cfg = %+v", parsed.cfg)
	}
	if got := parsed.origin("browser", "b", true); got != "flag" {
		t.Fatalf("origin = %q", got)
	}
}

func TestShorthandBeatsSettings(t *testing.T) {
	path := writeSettings(t, "browser = brave\n")

	parsed, err := parseFlags([]string{"--config", path, "-b", "edge"})
	if err != nil {
		t.Fatal(err)
	}

	if parsed.cfg.browser != "edge" {
		t.Fatalf("browser = %q", parsed.cfg.browser)
	}
}

func TestSettingsFileIsReported(t *testing.T) {
	path := writeSettings(t, "")

	parsed, err := parseFlags([]string{"--config", path})
	if err != nil {
		t.Fatal(err)
	}

	if parsed.file != path {
		t.Fatalf("file = %q, want %q", parsed.file, path)
	}
}

func TestBrokenSettingsStopTheRun(t *testing.T) {
	path := writeSettings(t, "browser = netscape\n")

	if _, err := parseFlags([]string{"--config", path}); err == nil {
		t.Fatal("expected the unknown browser to be rejected")
	}
}

func TestNegativeKeepIsRejected(t *testing.T) {
	path := writeSettings(t, "")

	if _, err := parseFlags([]string{"--config", path, "--keep", "-1"}); err == nil {
		t.Fatal("expected a negative keep to be rejected")
	}
}

func TestZeroKeepDisablesBackups(t *testing.T) {
	opts, err := fixerOptions(config{keep: 0})
	if err != nil {
		t.Fatal(err)
	}
	if opts.Backup {
		t.Fatal("--keep 0 should turn backups off")
	}

	opts, err = fixerOptions(config{keep: 3})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.Backup {
		t.Fatal("backups should be on by default")
	}
}

func TestUnknownBrowserIsRejected(t *testing.T) {
	if _, err := fixerOptions(config{browser: "netscape"}); err == nil {
		t.Fatal("expected an error")
	}
}

func TestQuietAndDryRunParse(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	parsed, err := parseFlags([]string{"-q", "-n"})
	if err != nil {
		t.Fatal(err)
	}

	if !parsed.cfg.quiet || !parsed.cfg.dryRun {
		t.Fatalf("cfg = %+v", parsed.cfg)
	}
	if parsed.cfg.source != "" {
		t.Fatalf("source = %q", parsed.cfg.source)
	}
}
