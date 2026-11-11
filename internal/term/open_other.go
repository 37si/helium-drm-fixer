//go:build !linux && !darwin && !windows

package term

func Open([]string, Options) error { return ErrUnavailable }
