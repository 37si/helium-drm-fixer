package helium

import (
	"errors"
	"os"
	"path/filepath"

	"hdf/internal/cdm"
)

var ErrNotFound = errors.New("Helium is not installed")

var ErrNoUserData = errors.New("Helium user data directory not found")

const HintFile = "latest-component-updated-widevine-cdm"

type Install struct {
	Root string
	Dir  string
	Exe  string
}

type CDM struct {
	Path    string
	Version string
}

func Find(override string) (Install, error) {
	install, err := locate(override)
	if err != nil {
		return Install{}, err
	}

	if install.Exe == "" {
		install.Exe = exe(install)
	}

	return install, nil
}

func locate(override string) (Install, error) {
	if override != "" {
		return fromOverride(override)
	}
	return find()
}

func Installed(install Install) []CDM {
	return installed(install)
}

func Root(install Install) (string, error) {
	return root(install)
}

func Target(install Install, cdmVersion string) (cdmPath, hintFile string, err error) {
	if err := cdm.ValidateVersion(cdmVersion); err != nil {
		return "", "", err
	}

	return target(install, cdmVersion)
}

func WriteHint(hintFile, cdmPath string) error {
	return writeHint(hintFile, cdmPath)
}

func fromOverride(dir string) (Install, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Install{}, err
	}

	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return Install{}, ErrNotFound
	}

	if install, ok := platformOverride(abs); ok {
		if install.Root == "" {
			return Install{}, ErrNotFound
		}
		return install, nil
	}

	if payload := payloadDir(abs); payload != "" {
		return Install{Root: abs, Dir: payload}, nil
	}

	return Install{Root: filepath.Dir(abs), Dir: abs}, nil
}
