//go:build linux || darwin

package disk

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func free(path string) (int64, error) {
	if _, err := os.Stat(path); err != nil {
		path = filepath.Dir(path)
	}

	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0, err
	}

	return int64(stat.Bavail) * int64(stat.Bsize), nil
}
