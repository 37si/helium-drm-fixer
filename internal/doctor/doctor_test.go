//go:build linux

package doctor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"hdf/internal/browser"
	"hdf/internal/helium"
)

func writeCDM(t *testing.T, dir, version string) string {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(dir, "_platform_specific", "linux_x64"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"version":"`+version+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "_platform_specific", "linux_x64", "libwidevinecdm.so"), []byte("so"), 0o644); err != nil {
		t.Fatal(err)
	}

	return dir
}

func fakeInstall(t *testing.T, version string) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	userData := filepath.Join(home, ".config", "net.imput.helium")
	writeCDM(t, filepath.Join(userData, "WidevineCdm", version), version)

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "helium"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	return root
}

func find(t *testing.T, checks []Check, name string) Check {
	t.Helper()

	for _, check := range checks {
		if check.Name == name {
			return check
		}
	}

	t.Fatalf("no %q check in %d results", name, len(checks))
	return Check{}
}

func TestRunSeesInstalledCDM(t *testing.T) {
	root := fakeInstall(t, "4.10.2900.0")

	checks := Run(context.Background(), Options{
		HeliumPath: root,
		StateDir:   t.TempDir(),
		BackupDir:  t.TempDir(),
	}, []browser.Detection{{
		Browser: browser.Browser{ID: browser.Brave, Name: "Brave"},
		CDM:     &browser.Location{Path: "/opt/brave/WidevineCdm", Version: "4.10.3112.0"},
	}})

	if got := find(t, checks, "Helium"); got.Level != LevelOK {
		t.Fatalf("Helium = %+v", got)
	}
	if got := find(t, checks, "WidevineCdm"); got.Level != LevelOK || got.Detail == "" {
		t.Fatalf("WidevineCdm = %+v", got)
	}
	if got := find(t, checks, "write access"); got.Level != LevelOK {
		t.Fatalf("write access = %+v", got)
	}
}

func TestRunFlagsForeignInstall(t *testing.T) {
	root := fakeInstall(t, "4.10.2900.0")

	checks := Run(context.Background(), Options{
		HeliumPath: root,
		StateDir:   t.TempDir(),
		BackupDir:  t.TempDir(),
	}, nil)

	if got := find(t, checks, "origin"); got.Level != LevelWarn {
		t.Fatalf("origin = %+v, want a warning", got)
	}
}

func TestRunFlagsOlderVersion(t *testing.T) {
	root := fakeInstall(t, "4.10.2900.0")

	checks := Run(context.Background(), Options{
		HeliumPath: root,
		StateDir:   t.TempDir(),
		BackupDir:  t.TempDir(),
	}, []browser.Detection{{
		Browser: browser.Browser{ID: browser.Brave, Name: "Brave"},
		CDM:     &browser.Location{Path: "/opt/brave/WidevineCdm", Version: "4.10.3112.0"},
	}})

	got := find(t, checks, "version")
	if got.Level != LevelWarn {
		t.Fatalf("version = %+v, want a warning", got)
	}
	if got.Hint == "" {
		t.Fatal("no hint on an outdated version")
	}
}

func TestRunWithoutHelium(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))

	checks := Run(context.Background(), Options{HeliumPath: filepath.Join(t.TempDir(), "missing")}, nil)

	if got := find(t, checks, "Helium"); got.Level != LevelFail {
		t.Fatalf("Helium = %+v", got)
	}
}

func TestSummaryCounts(t *testing.T) {
	fails, warns := Summary([]Check{
		{Level: LevelOK},
		{Level: LevelWarn},
		{Level: LevelFail},
		{Level: LevelInfo},
		{Level: LevelWarn},
	})

	if fails != 1 || warns != 2 {
		t.Fatalf("fails = %d, warns = %d", fails, warns)
	}
}

func TestInstalledFindsNewest(t *testing.T) {
	root := fakeInstall(t, "4.10.2900.0")
	home, _ := os.UserHomeDir()
	writeCDM(t, filepath.Join(home, ".config", "net.imput.helium", "WidevineCdm", "4.10.3112.0"), "4.10.3112.0")

	install, err := helium.Find(root)
	if err != nil {
		t.Fatal(err)
	}

	found := helium.Installed(install)
	if len(found) != 2 {
		t.Fatalf("found %d CDMs", len(found))
	}
	if found[0].Version != "4.10.3112.0" {
		t.Fatalf("newest = %q", found[0].Version)
	}
}
