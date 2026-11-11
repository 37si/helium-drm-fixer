package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hdf/internal/fixer"
	"hdf/internal/term"
)

func clearTermEnv(t *testing.T) {
	t.Helper()

	for _, key := range []string{
		term.LaunchedEnv, "TERM", "TERM_PROGRAM", "VSCODE_IPC_HOOK_CLI",
		"CURSOR_TRACE_ID", "WINDSURF_SESSION_ID", "ZED_TERM",
		"TERMINAL_EMULATOR", "JETBRAINS_IDE", "INSIDE_EMACS",
	} {
		t.Setenv(key, "")
	}
}

func TestWantsWindow(t *testing.T) {
	cases := []struct {
		name string
		cfg  config
		env  map[string]string
		want bool
	}{
		{"editor terminal", config{}, map[string]string{"TERM_PROGRAM": "vscode"}, true},
		{"jetbrains", config{}, map[string]string{"TERMINAL_EMULATOR": "JetBrains-JediTerm"}, true},
		{"real terminal", config{}, map[string]string{"TERM_PROGRAM": "iTerm.app"}, false},
		{"forced", config{external: true}, nil, true},
		{"disabled", config{noExternal: true}, map[string]string{"TERM_PROGRAM": "vscode"}, false},
		{"already in a window", config{external: true}, map[string]string{term.LaunchedEnv: "1"}, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			clearTermEnv(t)
			for key, value := range c.env {
				t.Setenv(key, value)
			}

			if got := wantsWindow(c.cfg); got != c.want {
				t.Fatalf("wantsWindow = %v, want %v", got, c.want)
			}
		})
	}
}

func TestNoExternalWinsOverExternal(t *testing.T) {
	clearTermEnv(t)

	cfg := config{external: true, noExternal: true}
	if wantsWindow(cfg) {
		t.Fatal("--no-external should win")
	}
}

func writeBackup(t *testing.T, dir, id string) string {
	t.Helper()

	path := filepath.Join(dir, id)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "meta.json"), []byte(`{"id":"`+id+`","bytes":10}`), 0o644); err != nil {
		t.Fatal(err)
	}

	return path
}

func TestClearBackupsDryRunKeepsSnapshots(t *testing.T) {
	dir := t.TempDir()
	path := writeBackup(t, dir, "20260101-000000.000")

	if err := clearBackups(config{dryRun: true}, fixer.Options{BackupDir: dir, Backups: 5}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("dry run removed a backup: %v", err)
	}
}

func TestClearBackupsWithYesRemovesSnapshots(t *testing.T) {
	dir := t.TempDir()
	path := writeBackup(t, dir, "20260101-000000.000")

	if err := clearBackups(config{yes: true}, fixer.Options{BackupDir: dir, Backups: 5}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("backup still exists: %v", err)
	}
}

func TestConfirmHelpers(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("y\nDELETE\n"))

	if !confirm(reader, "") {
		t.Fatal("y was not accepted")
	}
	if !confirmWord(reader, "", "DELETE") {
		t.Fatal("DELETE was not accepted")
	}

	if confirm(bufio.NewReader(strings.NewReader("n\n")), "") {
		t.Fatal("n was accepted")
	}
	if confirmWord(bufio.NewReader(strings.NewReader("delete\n")), "", "DELETE") {
		t.Fatal("wrong case was accepted")
	}
}

func TestTemporaryBuild(t *testing.T) {
	temp := t.TempDir()

	cases := map[string]bool{
		filepath.Join(temp, "go-build123", "b001", "exe", "hdf.exe"):            true,
		filepath.Join(temp, "hdf"):                                              true,
		filepath.Join(string(filepath.Separator), "usr", "local", "bin", "hdf"): false,
	}

	for path, want := range cases {
		if got := temporaryBuild(path); got != want {
			t.Fatalf("temporaryBuild(%q) = %v, want %v", path, got, want)
		}
	}
}
