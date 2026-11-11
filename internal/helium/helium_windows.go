package helium

import (
	"encoding/json"
	"os"
	"path/filepath"

	"hdf/internal/pathutil"
	"hdf/internal/proc"
)

func platformOverride(string) (Install, bool) { return Install{}, false }

func find() (Install, error) {
	if install, ok := fromRoots(); ok {
		return install, nil
	}

	if status := proc.Current(0).Helium(""); status.Running && len(status.Paths) > 0 {
		return fromExe(status.Paths[0]), nil
	}

	return Install{}, ErrNotFound
}

func fromExe(exe string) Install {
	app := filepath.Dir(exe)

	return layout(filepath.Dir(app), app, exe)
}

func fromRoots() (Install, bool) {
	for _, app := range installRoots() {
		exe := browserExe(app)
		if exe == "" && len(pathutil.VersionDirs(app)) == 0 {
			continue
		}

		return layout(filepath.Dir(app), app, exe), true
	}

	return Install{}, false
}

func installRoots() []string {
	var roots []string

	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		if home, err := os.UserHomeDir(); err == nil {
			local = filepath.Join(home, "AppData", "Local")
		}
	}

	if local != "" {
		roots = append(roots,

			filepath.Join(local, "imput", "Helium", "Application"),
			filepath.Join(local, "Helium", "Application"),
			filepath.Join(local, "Programs", "Helium", "Application"),
			filepath.Join(local, "Programs", "Helium"),
		)
	}

	for _, variable := range []string{"PROGRAMFILES", "PROGRAMFILES(X86)"} {
		dir := os.Getenv(variable)
		if dir == "" {
			continue
		}

		roots = append(roots,
			filepath.Join(dir, "imput", "Helium", "Application"),
			filepath.Join(dir, "Helium", "Application"),
			filepath.Join(dir, "Helium"),
		)
	}

	return roots
}

func userDataDir() (string, error) {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", ErrNoUserData
		}
		local = filepath.Join(home, "AppData", "Local")
	}

	candidates := []string{
		filepath.Join(local, "imput", "Helium", "User Data"),
		filepath.Join(local, "Helium", "User Data"),
		filepath.Join(local, "net.imput.helium"),
	}

	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir, nil
		}
	}

	for _, dir := range candidates {
		if info, err := os.Stat(filepath.Dir(dir)); err == nil && info.IsDir() {
			return dir, nil
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

func exe(install Install) string {
	if install.Exe != "" {
		return install.Exe
	}

	app := appDir(install)
	for _, dir := range []string{app, install.Dir, install.Root} {
		if found := browserExe(dir); found != "" {
			return found
		}
	}

	return ""
}

func installed(install Install) []CDM {
	var found []CDM

	if root, err := root(install); err == nil {
		if active, ok := hintedCDM(root); ok {
			found = append(found, active)
		}

		found = append(found, cdmsUnder(root)...)
	}

	app := appDir(install)
	found = append(found, cdmsUnder(filepath.Join(app, "WidevineCdm"))...)
	found = append(found, bundledCDMs(app)...)

	return dedupe(found)
}
