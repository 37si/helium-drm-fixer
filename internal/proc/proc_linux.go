//go:build linux

package proc

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func scan() Snapshot {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return Snapshot{At: now()}
	}

	procs := make([]Process, 0, len(entries))

	for _, entry := range entries {
		pid, ok := pid(entry.Name())
		if !ok {
			continue
		}

		raw, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "comm"))
		if err != nil {
			continue
		}

		name := strings.TrimSpace(string(raw))
		if name == "" {
			continue
		}

		item := Process{PID: pid, Name: name}

		if target, err := os.Readlink(filepath.Join("/proc", entry.Name(), "exe")); err == nil {
			item.Exe = strings.TrimSuffix(target, " (deleted)")
		}

		if looksHelium(name) {
			item.Args = cmdline(entry.Name())
		}

		procs = append(procs, item)
	}

	return Snapshot{At: now(), Procs: procs}
}

func cmdline(dir string) []string {
	raw, err := os.ReadFile(filepath.Join("/proc", dir, "cmdline"))
	if err != nil || len(raw) == 0 {
		return nil
	}

	parts := strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00")
	if len(parts) > 16 {
		parts = parts[:16]
	}

	return parts
}

func pid(name string) (int, bool) {
	if name == "" {
		return 0, false
	}

	for _, r := range name {
		if r < '0' || r > '9' {
			return 0, false
		}
	}

	value, err := strconv.Atoi(name)
	if err != nil || value <= 0 {
		return 0, false
	}

	return value, true
}
