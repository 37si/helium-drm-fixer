//go:build darwin

package term

import (
	"fmt"
	"os/exec"
)

func Open(argv []string, opts Options) error {
	script, err := writeScript("run.command", shScript(argv, opts.Dir))
	if err != nil {
		return err
	}

	app := opts.Terminal
	if app == "" {
		app = "Terminal"
	}

	if err := exec.Command("open", "-a", app, script).Run(); err != nil {
		return fmt.Errorf("%w: open -a %s: %w", ErrUnavailable, app, err)
	}

	return nil
}
