package helium

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"hdf/internal/cdm"
	"hdf/internal/pathutil"
)

func layout(root, app, exe string) Install {
	install := Install{Root: root, Dir: app, Exe: exe}

	if dir := payloadDir(app); dir != "" {
		install.Dir = dir
	}

	return install
}

func payloadDir(app string) string {
	versions := pathutil.VersionDirs(app)
	if len(versions) == 0 {
		return ""
	}

	for _, version := range versions {
		dir := filepath.Join(app, version)
		if hasPayload(dir) {
			return dir
		}
	}

	return filepath.Join(app, versions[0])
}

func hasPayload(dir string) bool {
	for _, name := range []string{"chrome.dll", "chrome.exe", "libchrome.so", "libchrome_dll.so"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}

	return false
}

func appDir(install Install) string {
	if install.Exe != "" {
		return filepath.Dir(install.Exe)
	}

	if pathutil.IsVersion(filepath.Base(install.Dir)) {
		return filepath.Dir(install.Dir)
	}

	return install.Dir
}

func browserExe(app string) string {
	for _, name := range []string{"chrome.exe", "new_chrome.exe", "helium.exe", "Helium.exe"} {
		path := filepath.Join(app, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}

	return ""
}

func cdmsUnder(root string) []CDM {
	if cdm.Valid(root) {
		return []CDM{newCDM(root)}
	}

	var out []CDM

	entries, err := os.ReadDir(root)
	if err != nil {
		return out
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dir := filepath.Join(root, entry.Name())
		if cdm.Valid(dir) {
			out = append(out, newCDM(dir))
		}
	}

	sortNewestFirst(out)

	return out
}

func sortNewestFirst(list []CDM) {
	sort.SliceStable(list, func(i, j int) bool {
		return pathutil.CompareVersions(list[i].Version, list[j].Version) > 0
	})
}

func bundledCDMs(app string) []CDM {
	entries, err := os.ReadDir(app)
	if err != nil {
		return nil
	}

	var out []CDM
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		out = append(out, cdmsUnder(filepath.Join(app, entry.Name(), "WidevineCdm"))...)
	}

	sortNewestFirst(out)

	return out
}

func newCDM(dir string) CDM {
	version, err := cdm.Version(dir)
	if err != nil || version == "" {
		version = filepath.Base(dir)
	}

	return CDM{Path: dir, Version: version}
}

func hintedCDM(root string) (CDM, bool) {
	raw, err := os.ReadFile(filepath.Join(root, HintFile))
	if err != nil {
		return CDM{}, false
	}

	var hint struct {
		Path string `json:"Path"`
	}
	if err := json.Unmarshal(raw, &hint); err != nil || hint.Path == "" {
		return CDM{}, false
	}

	if !cdm.Valid(hint.Path) {
		return CDM{}, false
	}

	version, _ := cdm.Version(hint.Path)

	return CDM{Path: hint.Path, Version: version}, true
}

func dedupe(list []CDM) []CDM {
	seen := make(map[string]struct{}, len(list))
	out := list[:0]

	for _, item := range list {
		if _, ok := seen[item.Path]; ok {
			continue
		}
		seen[item.Path] = struct{}{}
		out = append(out, item)
	}

	return out
}
