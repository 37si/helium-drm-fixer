package tui

import (
	"errors"

	"hdf/internal/fixer"
	"hdf/internal/helium"
)

func errText(err error) string {
	if err == nil {
		return "unknown error"
	}
	return err.Error()
}

func hintsFor(err error) []string {
	switch {
	case errors.Is(err, fixer.ErrNoSource):
		return []string{
			"install Chrome, Edge, Brave, Vivaldi, Opera GX or Chromium",
			"or point the tool at a CDM with --source-path",
		}
	case errors.Is(err, helium.ErrNotFound):
		return []string{
			"install Helium from https://helium.is/",
			"or pass --helium with its install directory",
		}
	case errors.Is(err, helium.ErrNoUserData):
		return []string{"launch Helium once so it creates its profile directory"}
	case errors.Is(err, fixer.ErrNoVersion):
		return []string{"the source CDM has no manifest.json with a version"}
	case errors.Is(err, fixer.ErrUnknownBrowser):
		return []string{"run --list to see the supported browser ids"}
	}

	return nil
}
