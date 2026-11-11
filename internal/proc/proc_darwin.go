//go:build darwin

package proc

import (
	"encoding/binary"
	"strings"

	"golang.org/x/sys/unix"
)

func scan() Snapshot {
	entries, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return Snapshot{At: now()}
	}

	procs := make([]Process, 0, len(entries))

	for i := range entries {
		item := Process{
			PID:  int(entries[i].Proc.P_pid),
			Name: cstring(entries[i].Proc.P_comm[:]),
		}

		if item.PID <= 0 || item.Name == "" {
			continue
		}

		if looksHelium(item.Name) {
			item.Exe, item.Args = args(item.PID)
		}

		procs = append(procs, item)
	}

	return Snapshot{At: now(), Procs: procs}
}

func args(pid int) (string, []string) {
	raw, err := unix.SysctlRaw("kern.procargs2", pid)
	if err != nil || len(raw) < 4 {
		return "", nil
	}

	argc := int(binary.NativeEndian.Uint32(raw[:4]))
	if argc <= 0 || argc > 4096 {
		return "", nil
	}

	rest := raw[4:]
	end := indexByte(rest, 0)
	if end <= 0 {
		return "", nil
	}

	exe := string(rest[:end])

	rest = rest[end:]
	for len(rest) > 0 && rest[0] == 0 {
		rest = rest[1:]
	}

	out := make([]string, 0, min(argc, 16))
	for len(rest) > 0 && len(out) < argc && len(out) < 16 {
		next := indexByte(rest, 0)
		if next <= 0 {
			break
		}
		out = append(out, string(rest[:next]))
		rest = rest[next+1:]
	}

	return exe, out
}

func indexByte(b []byte, want byte) int {
	for i, v := range b {
		if v == want {
			return i
		}
	}

	return -1
}

func cstring(b []byte) string {
	if i := indexByte(b, 0); i >= 0 {
		b = b[:i]
	}

	return strings.TrimSpace(string(b))
}
