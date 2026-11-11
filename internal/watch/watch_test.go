//go:build linux

package watch

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"charm.land/log/v2"

	"hdf/internal/fixer"
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

func environment(t *testing.T) (heliumRoot, source string) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	userData := filepath.Join(home, ".config", "net.imput.helium")
	if err := os.MkdirAll(userData, 0o755); err != nil {
		t.Fatal(err)
	}

	heliumRoot = t.TempDir()
	if err := os.WriteFile(filepath.Join(heliumRoot, "helium"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	source = writeCDM(t, filepath.Join(t.TempDir(), "WidevineCdm"), "4.10.3112.0")

	return heliumRoot, source
}

func watcher(t *testing.T, heliumRoot, source string) *Watcher {
	t.Helper()

	return New(Options{
		Interval: 5 * time.Second,
		Fixer: fixer.Options{
			SourcePath: source,
			HeliumPath: heliumRoot,
			Backup:     true,
			BackupDir:  t.TempDir(),
			StateDir:   t.TempDir(),
			Tool:       "test",
		},
	}, log.New(io.Discard))
}

func TestTickInstallsMissingCDM(t *testing.T) {
	root, source := environment(t)
	w := watcher(t, root, source)

	event, ok := w.tick(context.Background())
	if !ok {
		t.Fatal("no event for a missing CDM")
	}
	if event.Kind != KindApplied {
		t.Fatalf("kind = %v, message = %q", event.Kind, event.Message)
	}
	if event.Version != "4.10.3112.0" {
		t.Fatalf("version = %q", event.Version)
	}

	event, ok = w.tick(context.Background())
	if !ok || event.Kind != KindChecked {
		t.Fatalf("second tick: %v %q", event.Kind, event.Message)
	}

	if _, ok := w.tick(context.Background()); ok {
		t.Fatal("a settled watcher kept reporting")
	}
}

func TestTickErrorsWithoutSource(t *testing.T) {
	root, _ := environment(t)

	w := New(Options{Interval: 5 * time.Second, Fixer: fixer.Options{
		HeliumPath: root,
		SourcePath: filepath.Join(t.TempDir(), "missing"),
	}}, log.New(io.Discard))

	event, ok := w.tick(context.Background())
	if !ok || event.Kind != KindError {
		t.Fatalf("kind = %v", event.Kind)
	}

	if _, ok := w.tick(context.Background()); ok {
		t.Fatal("the same error was reported twice")
	}
}

func TestDryRunDoesNotWrite(t *testing.T) {
	root, source := environment(t)
	w := watcher(t, root, source)
	w.opts.DryRun = true

	event, ok := w.tick(context.Background())
	if !ok || event.Kind != KindApplied {
		t.Fatalf("kind = %v", event.Kind)
	}

	home, _ := os.UserHomeDir()
	installed := filepath.Join(home, ".config", "net.imput.helium", "WidevineCdm", "4.10.3112.0")
	if _, err := os.Stat(installed); err == nil {
		t.Fatal("a dry run wrote the CDM")
	}
}

func TestIntervalIsFloored(t *testing.T) {
	w := New(Options{Interval: time.Second}, log.New(io.Discard))
	if w.opts.Interval != MinInterval {
		t.Fatalf("interval = %v, want %v", w.opts.Interval, MinInterval)
	}
}
