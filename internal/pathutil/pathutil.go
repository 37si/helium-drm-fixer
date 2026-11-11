package pathutil

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var windowsVar = regexp.MustCompile(`%([^%]+)%`)

func Expand(tmpl string) (string, bool) {
	if home, err := os.UserHomeDir(); err == nil {
		switch {
		case tmpl == "~":
			return home, true
		case strings.HasPrefix(tmpl, "~/"):
			tmpl = filepath.Join(home, strings.TrimPrefix(tmpl, "~/"))
		}
	}

	tmpl = windowsVar.ReplaceAllString(tmpl, `${$1}`)

	var missing bool

	out := os.Expand(tmpl, func(key string) string {
		value, ok := os.LookupEnv(key)
		if !ok {
			missing = true
			return ""
		}
		return value
	})

	if missing {
		return "", false
	}

	return out, true
}

func ExpandAll(templates []string) []string {
	out := make([]string, 0, len(templates))

	for _, tmpl := range templates {
		if value, ok := Expand(tmpl); ok && value != "" {
			out = append(out, value)
		}
	}

	return out
}

const LegacyDirName = "helium-drm"

func MigrateDir(dir string) bool {
	if dir == "" {
		return false
	}

	if _, err := os.Lstat(dir); err == nil {
		return false
	}

	legacy := filepath.Join(filepath.Dir(dir), LegacyDirName)
	if legacy == dir {
		return false
	}

	info, err := os.Lstat(legacy)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false
	}

	if err := os.Rename(legacy, dir); err != nil {
		return false
	}

	return true
}

func VersionDirs(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}

	var versions []string
	for _, entry := range entries {
		if entry.IsDir() && IsVersion(entry.Name()) {
			versions = append(versions, entry.Name())
		}
	}

	sort.Slice(versions, func(i, j int) bool {
		return CompareVersions(versions[i], versions[j]) > 0
	})

	return versions
}

func IsVersion(name string) bool {
	parts := strings.Split(name, ".")
	if len(parts) < 2 || len(parts) > 4 {
		return false
	}

	for _, part := range parts {
		if part == "" {
			return false
		}

		for _, r := range part {
			if r < '0' || r > '9' {
				return false
			}
		}
	}

	return true
}

func CompareVersions(a, b string) int {
	left, right := strings.Split(a, "."), strings.Split(b, ".")
	parts := max(len(left), len(right))

	for i := 0; i < parts; i++ {
		var l, r int
		if i < len(left) {
			l, _ = strconv.Atoi(left[i])
		}
		if i < len(right) {
			r, _ = strconv.Atoi(right[i])
		}

		switch {
		case l < r:
			return -1
		case l > r:
			return 1
		}
	}

	return 0
}
