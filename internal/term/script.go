package term

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Options struct {
	Terminal string
	Dir      string
}

func shScript(argv []string, dir string) string {
	var b strings.Builder

	b.WriteString("#!/bin/sh\n")
	fmt.Fprintf(&b, "export %s=1\n", LaunchedEnv)

	if dir != "" {
		fmt.Fprintf(&b, "cd %s || exit 1\n", shQuote(dir))
	}

	b.WriteString(shJoin(argv) + "\n")
	b.WriteString("status=$?\n")
	b.WriteString("if [ \"$status\" -ne 0 ]; then\n")
	b.WriteString("  printf '\\npress enter to close'\n")
	b.WriteString("  read -r _\n")
	b.WriteString("fi\n")
	b.WriteString("exit \"$status\"\n")

	return b.String()
}

func batScript(argv []string, dir string) string {
	var b strings.Builder

	b.WriteString("@echo off\r\n")
	fmt.Fprintf(&b, "set %s=1\r\n", LaunchedEnv)

	if dir != "" {
		fmt.Fprintf(&b, "if exist %s cd /d %s\r\n", batQuote(dir), batQuote(dir))
	}

	b.WriteString(batJoin(argv) + "\r\n")
	b.WriteString("if errorlevel 1 (\r\n")
	b.WriteString("  echo.\r\n")
	b.WriteString("  echo hdf stopped, the error is above.\r\n")
	b.WriteString("  pause\r\n")
	b.WriteString(")\r\n")

	return b.String()
}

func CleanupScripts() {
	entries, err := os.ReadDir(os.TempDir())
	if err != nil {
		return
	}

	cutoff := time.Now().Add(-24 * time.Hour)
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "hdf-launch-") {
			continue
		}

		info, err := entry.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}

		os.RemoveAll(filepath.Join(os.TempDir(), entry.Name()))
	}
}

func writeScript(name, body string) (string, error) {
	dir, err := os.MkdirTemp("", "hdf-launch-")
	if err != nil {
		return "", err
	}

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
		return "", err
	}

	return path, nil
}

func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func shJoin(argv []string) string {
	quoted := make([]string, len(argv))
	for i, arg := range argv {
		quoted[i] = shQuote(arg)
	}

	return strings.Join(quoted, " ")
}

func batQuote(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func batJoin(argv []string) string {
	quoted := make([]string, len(argv))
	for i, arg := range argv {
		quoted[i] = batQuote(arg)
	}

	return strings.Join(quoted, " ")
}
