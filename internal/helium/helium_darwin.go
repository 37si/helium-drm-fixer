package helium

import (
	"os"
	"path/filepath"
	"strings"

	"hdf/internal/cdm"
	"hdf/internal/pathutil"
	"hdf/internal/proc"
)

const AppPath = "/Applications/Helium.app"

var appBundles = []string{
	AppPath,
	"~/Applications/Helium.app",
	"/Applications/Helium Beta.app",
	"~/Applications/Helium Beta.app",
}

func find() (Install, error) {
	for _, bundle := range expand(appBundles) {
		if install, ok := fromBundle(bundle); ok {
			return install, nil
		}
	}

	if status := proc.Current(0).Helium(""); status.Running && len(status.Paths) > 0 {
		if install, ok := fromBundle(bundleFor(status.Paths[0])); ok {
			return install, nil
		}
	}

	return Install{}, ErrNotFound
}

func platformOverride(path string) (Install, bool) {
	for probe := path; ; probe = filepath.Dir(probe) {
		if strings.EqualFold(filepath.Ext(probe), ".app") {
			install, _ := fromBundle(probe)
			return install, true
		}

		parent := filepath.Dir(probe)
		if parent == probe {
			return Install{}, false
		}
	}
}

func fromBundle(bundle string) (Install, bool) {
	if bundle == "" {
		return Install{}, false
	}

	framework := filepath.Join(bundle, "Contents", "Frameworks", "Helium Framework.framework")
	versionsRoot := filepath.Join(framework, "Versions")

	dirs := pathutil.VersionDirs(versionsRoot)
	if len(dirs) == 0 {
		if info, err := os.Stat(filepath.Join(versionsRoot, "Current", "Libraries")); err == nil && info.IsDir() {
			return Install{Root: bundle, Dir: filepath.Join(versionsRoot, "Current", "Libraries")}, true
		}
		return Install{}, false
	}

	return Install{
		Root: bundle,
		Dir:  filepath.Join(versionsRoot, dirs[0], "Libraries"),
	}, true
}

func bundleFor(exe string) string {
	dir := filepath.Dir(exe)

	for i := 0; i < 6; i++ {
		if filepath.Ext(dir) == ".app" {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return ""
}

func expand(paths []string) []string {
	out := make([]string, 0, len(paths))
	home, _ := os.UserHomeDir()

	for _, path := range paths {
		if len(path) > 2 && path[:2] == "~/" && home != "" {
			path = filepath.Join(home, path[2:])
		}
		out = append(out, path)
	}

	return out
}

func userDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", ErrNoUserData
	}

	for _, dir := range []string{
		filepath.Join(home, "Library", "Application Support", "net.imput.helium"),
		filepath.Join(home, "Library", "Application Support", "Helium"),
	} {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir, nil
		}
	}

	return "", ErrNoUserData
}

func exe(install Install) string {
	var candidates []string

	if install.Root != "" {
		name := filepath.Base(install.Root)
		if strings.EqualFold(filepath.Ext(name), ".app") {
			name = strings.TrimSuffix(name, filepath.Ext(name))
			candidates = append(candidates, filepath.Join(install.Root, "Contents", "MacOS", name))
		}
		candidates = append(candidates, filepath.Join(install.Root, "Contents", "MacOS", "Helium"))
	}

	candidates = append(candidates, filepath.Join(AppPath, "Contents", "MacOS", "Helium"))

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}

	return ""
}

func installed(install Install) []CDM {
	dir := filepath.Join(install.Dir, "WidevineCdm")
	if !cdm.Valid(dir) {
		return nil
	}

	version, _ := cdm.Version(dir)

	return []CDM{{Path: dir, Version: version}}
}

func root(install Install) (string, error) {
	return filepath.Join(install.Dir, "WidevineCdm"), nil
}

func target(install Install, _ string) (string, string, error) {
	return filepath.Join(install.Dir, "WidevineCdm"), "", nil
}

func writeHint(string, string) error { return nil }
