//go:build !linux && !darwin && !windows

package disk

import "errors"

func free(string) (int64, error) { return 0, errors.New("free space is not available here") }
