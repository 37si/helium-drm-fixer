package proc

import (
	"path/filepath"
	"strings"
)

func normalize(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.TrimSuffix(name, ".exe")
	name = strings.TrimSuffix(name, ".app")

	return name
}

func base(path string) string {
	if index := strings.LastIndexAny(path, `\/`); index >= 0 {
		return path[index+1:]
	}

	return path
}

var chromiumNames = map[string]struct{}{
	"chrome":              {},
	"new_chrome":          {},
	"chrome_proxy":        {},
	"chrome_pwa_launcher": {},
	"chromium":            {},
}

func chromiumName(name string) bool {
	_, ok := chromiumNames[normalize(name)]

	return ok
}

func heliumPath(path string) bool {
	if path == "" {
		return false
	}

	path = strings.ToLower(strings.ReplaceAll(path, `\`, "/"))

	return strings.Contains(path, "/imput/helium/") ||
		strings.Contains(path, "/helium/application/")
}

func matches(name string, want []string) bool {
	name = normalize(name)
	if name == "" {
		return false
	}

	for _, candidate := range want {
		if normalize(candidate) == name {
			return true
		}
	}

	return false
}

func canonical(path string) string {
	path = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(path), " (deleted)"))
	if path == "" {
		return ""
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}

	return filepath.Clean(abs)
}
