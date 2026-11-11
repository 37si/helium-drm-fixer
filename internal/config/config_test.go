package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func write(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	return path
}

func TestLoadMissingIsNotAnError(t *testing.T) {
	settings, err := Load(filepath.Join(t.TempDir(), FileName))
	if err != nil {
		t.Fatal(err)
	}
	if settings.Browser != "" || settings.Keep != nil {
		t.Fatalf("settings = %+v", settings)
	}
}

func TestLoadReadsEveryKey(t *testing.T) {
	path := write(t, `# hdf settings
browser = brave
helium  = /opt/helium
repo    = acme/hdf
interval = 2m
keep = 3
newest = true
no-backup = yes
external = on
`)

	settings, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if settings.Browser != "brave" || settings.Helium != "/opt/helium" || settings.Repo != "acme/hdf" {
		t.Fatalf("settings = %+v", settings)
	}
	if settings.Keep == nil || *settings.Keep != 3 {
		t.Fatalf("keep = %v", settings.Keep)
	}
	if settings.Newest == nil || !*settings.Newest {
		t.Fatal("newest was not read")
	}
	if settings.NoBackup == nil || !*settings.NoBackup {
		t.Fatal("no-backup was not read")
	}
	if settings.External == nil || !*settings.External {
		t.Fatal("external was not read")
	}
	if value, ok := settings.PollInterval(); !ok || value != 2*time.Minute {
		t.Fatalf("interval = %v %v", value, ok)
	}
}

func TestLoadKeepsQuotedValues(t *testing.T) {
	settings, err := Load(write(t, "helium = \"/Applications/Helium Beta.app\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if settings.Helium != "/Applications/Helium Beta.app" {
		t.Fatalf("helium = %q", settings.Helium)
	}
}

func TestLoadRejectsBadValues(t *testing.T) {
	cases := map[string]string{
		"browser":   "browser = netscape\n",
		"interval":  "interval = soon\n",
		"keep":      "keep = -1\n",
		"keep text": "keep = lots\n",
		"boolean":   "newest = maybe\n",
		"unknown":   "nonsense = 1\n",
		"no equals": "browser chrome\n",
		"json":      `{"browser":`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(write(t, body)); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestLoadAcceptsEmptyFile(t *testing.T) {
	settings, err := Load(write(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	if settings.Keep != nil {
		t.Fatalf("keep = %v", settings.Keep)
	}
}

func TestPathIsEmptyWithoutADirectory(t *testing.T) {
	if Path("") != "" {
		t.Fatal("expected an empty path")
	}
}

func TestFileNameIsACfg(t *testing.T) {
	if FileName != "config.cfg" {
		t.Fatalf("FileName = %q", FileName)
	}
}
