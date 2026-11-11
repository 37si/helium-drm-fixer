package tui

import (
	"context"
	"os"

	tea "charm.land/bubbletea/v2"
	"charm.land/log/v2"
	"github.com/charmbracelet/colorprofile"
)

func Run(ctx context.Context, opts Options, logger *log.Logger) error {
	Configure(colorprofile.Detect(os.Stdout, os.Environ()), true, nil)

	model := New(ctx, opts, logger)

	program := tea.NewProgram(model, tea.WithContext(ctx))
	if _, err := program.Run(); err != nil {
		return err
	}

	return nil
}
