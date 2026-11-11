//go:build !windows

package proc

func foldPath(path string) string { return path }
