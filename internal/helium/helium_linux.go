package helium

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"

	"hdf/internal/pathutil"
	"hdf/internal/proc"
)

var linuxBinaries = []string{
	"/usr/bin/helium",
	"/usr/bin/helium-bin",
	"/usr/bin/helium-browser",
	"/usr/local/bin/helium",
	"/opt/helium/helium",
	"/opt/helium-bin/helium",
	"/opt/helium-browser/helium",
	"/opt/helium-browser-bin/helium",
}

var linuxSystemRoots = []string{
	"/opt/helium",
	"/opt/helium-bin",
	"/opt/helium-browser",
	"/opt/helium-browser-bin",
	"/usr/lib/helium",
	"/usr/lib64/helium",
	"/usr/lib/helium-bin",
}

func platformOverride(string) (Install, bool) { return Install{}, false }

func find() (Install, error) {
	if install, ok := fromBinaries(); ok {
		return install, nil
	}

	if install, ok := fromRoots(); ok {
		return install, nil
	}

	if install, ok := fromRunning(); ok {
		return install, nil
	}

	return Install{}, ErrNotFound
}

func fromBinaries() (Install, bool) {
	for _, binary := range linuxBinaries {
		resolved, err := filepath.EvalSymlinks(binary)
		if err != nil {
			continue
		}

		dir := filepath.Dir(resolved)
		if hasHeliumBinary(dir) {
			return Install{Root: dir, Dir: dir}, true
		}
	}

	for _, name := range []string{"helium", "helium-bin", "helium-browser"} {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}

		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			continue
		}

		dir := filepath.Dir(resolved)
		if hasHeliumBinary(dir) {
			return Install{Root: dir, Dir: dir}, true
		}
	}

	return Install{}, false
}

func fromRoots() (Install, bool) {
	for _, root := range linuxSystemRoots {
		if hasHeliumBinary(root) {
			return Install{Root: root, Dir: root}, true
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return Install{}, false
	}

	for _, root := range []string{
		filepath.Join(home, ".local", "share", "helium"),
		filepath.Join(home, ".local", "opt", "helium"),
		filepath.Join(home, ".config", "Helium", "Application"),
	} {
		if versions := pathutil.VersionDirs(root); len(versions) > 0 {
			return Install{Root: root, Dir: filepath.Join(root, versions[0])}, true
		}
		if hasHeliumBinary(root) {
			return Install{Root: root, Dir: root}, true
		}
	}

	return Install{}, false
}

func fromRunning() (Install, bool) {
	status := proc.Current(0).Helium("")
	if !status.Running || len(status.Paths) == 0 {
		return Install{}, false
	}

	exe := status.Paths[0]
	dir := filepath.Dir(exe)

	return Install{Root: dir, Dir: dir, Exe: exe}, true
}

func hasHeliumBinary(dir string) bool {
	for _, name := range []string{"helium", "helium-bin", "helium-wrapper", "helium-browser"} {
		if info, err := os.Stat(filepath.Join(dir, name)); err == nil && !info.IsDir() {
			return true
		}
	}

	return false
}

func exe(install Install) string {
	if install.Exe != "" {
		return install.Exe
	}

	candidates := []string{
		filepath.Join(install.Dir, "helium"),
		filepath.Join(install.Root, "helium"),
		filepath.Join(install.Dir, "helium-bin"),
		filepath.Join(install.Root, "helium-bin"),
	}
	candidates = append(candidates, linuxBinaries...)

	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}

		if resolved, err := filepath.EvalSymlinks(candidate); err == nil {
			return resolved
		}

		return candidate
	}

	return ""
}

func installed(install Install) []CDM {
	userData, err := userDataDir()
	if err != nil {
		return nil
	}

	root := filepath.Join(userData, "WidevineCdm")

	var found []CDM
	if active, ok := hintedCDM(root); ok {
		found = append(found, active)
	}

	return dedupe(append(found, cdmsUnder(root)...))
}

func userDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", ErrNoUserData
	}

	config := os.Getenv("XDG_CONFIG_HOME")
	if config == "" {
		config = filepath.Join(home, ".config")
	}

	candidates := []string{
		filepath.Join(config, "net.imput.helium"),
		filepath.Join(config, "Helium"),
		filepath.Join(config, "helium"),
	}

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
	}

	return "", ErrNoUserData
}

func root(install Install) (string, error) {
	userData, err := userDataDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(userData, "WidevineCdm"), nil
}

func target(install Install, cdmVersion string) (string, string, error) {
	root, err := root(install)
	if err != nil {
		return "", "", err
	}

	return filepath.Join(root, cdmVersion), filepath.Join(root, HintFile), nil
}

func writeHint(hintFile, cdmPath string) error {
	if err := os.MkdirAll(filepath.Dir(hintFile), 0o755); err != nil {
		return err
	}

	payload, err := json.Marshal(map[string]string{"Path": cdmPath})
	if err != nil {
		return err
	}

	return os.WriteFile(hintFile, payload, 0o644)
}
