//go:build darwin

package fixer

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func needsResign() bool { return true }

func resign(ctx context.Context, appPath, target string) error {
	if appPath == "" {
		appPath = appRootFor(target)
	}
	if appPath == "" {
		appPath = "/Applications/Helium.app"
	}

	if out, err := exec.CommandContext(ctx, "xattr", "-cr", appPath).CombinedOutput(); err != nil {
		return fmt.Errorf("xattr: %w: %s", err, out)
	}

	if out, err := exec.CommandContext(ctx, "codesign", "--force", "--deep", "--sign", "-", appPath).CombinedOutput(); err != nil {
		return fmt.Errorf("codesign: %w: %s", err, out)
	}

	return nil
}

func appRootFor(path string) string {
	for probe := path; ; probe = filepath.Dir(probe) {
		if strings.EqualFold(filepath.Ext(probe), ".app") {
			return probe
		}

		parent := filepath.Dir(probe)
		if parent == probe {
			return ""
		}
	}
}
