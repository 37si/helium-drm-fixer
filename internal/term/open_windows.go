//go:build windows

package term

import (
	"fmt"
	"os/exec"
)

func Open(argv []string, opts Options) error {
	script, err := writeScript("run.bat", batScript(argv, opts.Dir))
	if err != nil {
		return err
	}

	if opts.Terminal != "" {
		path, err := exec.LookPath(opts.Terminal)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrUnavailable, opts.Terminal)
		}

		if err := exec.Command(path, "cmd.exe", "/c", script).Start(); err != nil {
			return fmt.Errorf("%w: %s: %w", ErrUnavailable, opts.Terminal, err)
		}

		return nil
	}

	if err := exec.Command("cmd.exe", "/c", "start", "", script).Run(); err != nil {
		return fmt.Errorf("%w: start: %w", ErrUnavailable, err)
	}

	return nil
}
