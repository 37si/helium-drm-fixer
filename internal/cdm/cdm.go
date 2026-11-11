package cdm

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var dirNames = []string{"WidevineCdm", "WidevineCdm.plugin"}

var markers = []string{"manifest.json", "_platform_specific", "WidevineCdm.plugin"}

var ErrInvalid = errors.New("not a WidevineCdm directory")

var ErrUnsafeVersion = errors.New("unusable WidevineCdm version")

const maxVersionLen = 64

func ValidateVersion(v string) error {
	if v == "" {
		return fmt.Errorf("%w: empty", ErrUnsafeVersion)
	}
	if len(v) > maxVersionLen {
		return fmt.Errorf("%w: %d characters", ErrUnsafeVersion, len(v))
	}
	if strings.ContainsAny(v, `/\:`) {
		return fmt.Errorf("%w: %q contains a path separator", ErrUnsafeVersion, v)
	}
	if v == "." || v == ".." || strings.HasPrefix(v, ".") {
		return fmt.Errorf("%w: %q is a relative component", ErrUnsafeVersion, v)
	}

	for _, r := range v {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case r == '.' || r == '_' || r == '-' || r == '+':
		default:
			return fmt.Errorf("%w: %q contains %q", ErrUnsafeVersion, v, r)
		}
	}

	return nil
}

func DirNames() []string {
	out := make([]string, len(dirNames))
	copy(out, dirNames)
	return out
}

func Valid(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}

	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return true
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		name := strings.ToLower(entry.Name())
		if !strings.HasPrefix(name, "widevinecdm.") && !strings.HasPrefix(name, "libwidevinecdm.") {
			continue
		}
		switch filepath.Ext(name) {
		case ".dll", ".so", ".dylib":
			return true
		}
	}

	return false
}

type manifest struct {
	Version string `json:"version"`
}

func Version(dir string) (string, error) {
	candidates := []string{filepath.Join(dir, "manifest.json")}

	if entries, err := os.ReadDir(dir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				candidates = append(candidates, filepath.Join(dir, entry.Name(), "manifest.json"))
			}
		}
	}

	for _, candidate := range candidates {
		raw, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}

		var m manifest
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		if m.Version != "" {
			if err := ValidateVersion(m.Version); err != nil {
				return "", fmt.Errorf("%s: %w", candidate, err)
			}

			return m.Version, nil
		}
	}

	return "", fmt.Errorf("%w: no manifest.json with a version in %s", ErrInvalid, dir)
}

var ErrOverlap = errors.New("source and destination overlap")

func Copy(src, dst string) error {
	if !Valid(src) {
		return fmt.Errorf("%w: %s", ErrInvalid, src)
	}

	if err := checkSeparate(src, dst); err != nil {
		return err
	}

	walkRoot := src
	if resolved, err := filepath.EvalSymlinks(src); err == nil {
		walkRoot = resolved
	}

	parent := filepath.Dir(dst)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create destination: %w", err)
	}

	stage, err := os.MkdirTemp(parent, ".hdf-copy-*")
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}
	defer os.RemoveAll(stage)

	mode := os.FileMode(0o755)
	if info, err := os.Stat(walkRoot); err == nil {
		mode = info.Mode().Perm() &^ 0o022
	}

	if err := copyTree(walkRoot, stage); err != nil {
		return err
	}

	if err := os.Chmod(stage, mode); err != nil {
		return fmt.Errorf("create destination: %w", err)
	}

	return replaceDir(stage, dst)
}

func copyTree(src, dst string) error {
	root, err := filepath.EvalSymlinks(src)
	if err != nil {
		root = src
	}

	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		target := filepath.Join(dst, rel)

		switch {
		case entry.Type()&os.ModeSymlink != 0:
			return copyLink(path, target, root)
		case entry.IsDir():
			info, err := entry.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(target, info.Mode().Perm()&^0o022)
		default:
			return copyFile(path, target, entry)
		}
	})
}

func replaceDir(stage, dst string) error {
	backup := ""

	if _, err := os.Lstat(dst); err == nil {
		reserved, err := os.MkdirTemp(filepath.Dir(dst), ".hdf-old-*")
		if err != nil {
			return fmt.Errorf("replace destination: %w", err)
		}
		os.Remove(reserved)
		backup = reserved

		if err := os.Rename(dst, backup); err != nil {
			return fmt.Errorf("replace destination: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("replace destination: %w", err)
	}

	if err := os.Rename(stage, dst); err != nil {
		if backup != "" {
			if restore := os.Rename(backup, dst); restore != nil {
				return fmt.Errorf("replace destination: %w (and could not put the old directory back: %v)", err, restore)
			}
		}
		return fmt.Errorf("replace destination: %w", err)
	}

	if backup != "" {
		os.RemoveAll(backup)
	}

	return nil
}

func checkSeparate(src, dst string) error {
	if Overlaps(src, dst) {
		return fmt.Errorf("%w: %s and %s", ErrOverlap, src, dst)
	}

	return nil
}

func Overlaps(a, b string) bool {
	x, y := resolved(a), resolved(b)

	return x == y || within(x, y) || within(y, x)
}

func resolved(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}

	for probe := path; ; probe = filepath.Dir(probe) {
		real, err := filepath.EvalSymlinks(probe)
		if err == nil {
			rest, relErr := filepath.Rel(probe, path)
			if relErr != nil || rest == "." {
				return real
			}
			return filepath.Join(real, rest)
		}

		if parent := filepath.Dir(probe); parent == probe {
			return path
		}
	}
}

func within(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil || rel == "." {
		return false
	}

	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func copyLink(path, target, root string) error {
	link, err := os.Readlink(path)
	if err != nil {
		return err
	}

	destination := link
	if !filepath.IsAbs(destination) {
		destination = filepath.Join(filepath.Dir(path), destination)
	}

	if !within(root, resolved(destination)) && resolved(destination) != root {
		return fmt.Errorf("%w: %s points at %s", ErrInvalid, filepath.Base(path), link)
	}

	return os.Symlink(link, target)
}

func copyFile(src, dst string, entry fs.DirEntry) error {
	info, err := entry.Info()
	if err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm()&^0o022)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}

	return out.Close()
}
