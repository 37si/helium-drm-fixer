//go:build !windows

package update

import (
	"errors"
	"fmt"
	"os"
)

func Replace(target, src string) error {
	if err := os.Chmod(src, 0o755); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if err := os.Rename(src, target); err != nil {
		return fmt.Errorf("replace %s: %w", target, err)
	}

	return nil
}
