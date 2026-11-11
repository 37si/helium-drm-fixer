//go:build windows

package proc

import "strings"

func foldPath(path string) string { return strings.ToLower(path) }
