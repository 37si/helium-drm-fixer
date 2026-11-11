package disk

import (
	"path/filepath"

	"golang.org/x/sys/windows"
)

func free(path string) (int64, error) {
	root := filepath.VolumeName(path) + `\`

	target, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return 0, err
	}

	var available, total, free uint64
	if err := windows.GetDiskFreeSpaceEx(target, &available, &total, &free); err != nil {
		return 0, err
	}

	return int64(available), nil
}
