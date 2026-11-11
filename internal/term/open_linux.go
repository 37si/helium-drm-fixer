//go:build linux

package term

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const settle = 400 * time.Millisecond

type emulator struct {
	bin  string
	args func(script string) []string
}

var emulators = []emulator{
	{"x-terminal-emulator", func(s string) []string { return []string{"-e", "sh", s} }},
	{"gnome-terminal", func(s string) []string { return []string{"--", "sh", s} }},
	{"konsole", func(s string) []string { return []string{"-e", "sh", s} }},
	{"xfce4-terminal", func(s string) []string { return []string{"-x", "sh", s} }},
	{"tilix", func(s string) []string { return []string{"-e", "sh " + s} }},
	{"mate-terminal", func(s string) []string { return []string{"-x", "sh", s} }},
	{"terminator", func(s string) []string { return []string{"-x", "sh", s} }},
	{"lxterminal", func(s string) []string { return []string{"-e", "sh " + s} }},
	{"kitty", func(s string) []string { return []string{"sh", s} }},
	{"alacritty", func(s string) []string { return []string{"-e", "sh", s} }},
	{"wezterm", func(s string) []string { return []string{"start", "--", "sh", s} }},
	{"foot", func(s string) []string { return []string{"sh", s} }},
	{"xterm", func(s string) []string { return []string{"-e", "sh", s} }},
}

func Open(argv []string, opts Options) error {
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return fmt.Errorf("%w: no display server", ErrUnavailable)
	}

	script, err := writeScript("run.sh", shScript(argv, opts.Dir))
	if err != nil {
		return err
	}

	wanted := ""
	if opts.Terminal != "" {
		wanted = filepath.Base(opts.Terminal)
	}

	tried := make([]string, 0, len(emulators))

	for _, em := range emulators {
		if wanted != "" && em.bin != wanted {
			continue
		}

		path, err := exec.LookPath(em.bin)
		if err != nil {
			tried = append(tried, em.bin)
			continue
		}

		if err := launch(path, em.args(script)); err != nil {
			tried = append(tried, em.bin)
			continue
		}

		return nil
	}

	if wanted != "" {
		return openUnknown(wanted, script, opts.Terminal)
	}

	return fmt.Errorf("%w: tried %s", ErrUnavailable, strings.Join(tried, ", "))
}

func openUnknown(bin, script, asked string) error {
	for _, candidate := range []string{asked, bin} {
		path, err := exec.LookPath(candidate)
		if err != nil {
			continue
		}
		if err := launch(path, []string{"-e", "sh", script}); err == nil {
			return nil
		}
	}

	return fmt.Errorf("%w: %s", ErrUnavailable, asked)
}

func launch(path string, args []string) error {
	cmd := exec.Command(path, args...)
	if err := cmd.Start(); err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		return err
	case <-time.After(settle):
		return nil
	}
}
