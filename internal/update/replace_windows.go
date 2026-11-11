package update

import (
	"fmt"
	"os"
)

func Replace(target, src string) error {
	old := target + ".old"

	os.Remove(old)

	if err := os.Rename(target, old); err != nil {
		return fmt.Errorf("move %s aside: %w", target, err)
	}

	if err := os.Rename(src, target); err != nil {
		if restore := os.Rename(old, target); restore != nil {
			return fmt.Errorf("install %s: %w (and could not restore the previous build: %v)", target, err, restore)
		}
		return fmt.Errorf("install %s: %w", target, err)
	}

	_ = os.Remove(old)

	return nil
}
