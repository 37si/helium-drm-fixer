package term

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestShQuote(t *testing.T) {
	cases := map[string]string{
		"/usr/bin/hdf":      `'/usr/bin/hdf'`,
		"/tmp/with space/x": `'/tmp/with space/x'`,
		"it's":              `'it'\''s'`,
		"":                  `''`,
	}

	for in, want := range cases {
		if got := shQuote(in); got != want {
			t.Fatalf("shQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestShScriptRunsInDirectoryAndHolds(t *testing.T) {
	body := shScript([]string{"/bin/hdf", "--browser", "edge"}, "/home/me")

	for _, want := range []string{
		"#!/bin/sh",
		"export " + LaunchedEnv + "=1",
		"cd '/home/me' || exit 1",
		`'/bin/hdf' '--browser' 'edge'`,
		"read -r _",
		`exit "$status"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("script is missing %q:\n%s", want, body)
		}
	}
}

func TestShScriptWithoutDirectory(t *testing.T) {
	if body := shScript([]string{"hdf"}, ""); strings.Contains(body, "cd ") {
		t.Fatalf("script should not cd:\n%s", body)
	}
}

func TestBatScript(t *testing.T) {
	body := batScript([]string{`C:\Program Files\hdf.exe`}, `C:\Users\me`)

	for _, want := range []string{
		"@echo off\r\n",
		"set " + LaunchedEnv + "=1",
		`if exist "C:\Users\me" cd /d "C:\Users\me"`,
		`"C:\Program Files\hdf.exe"`,
		"pause",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("script is missing %q:\n%s", want, body)
		}
	}
}

func TestWriteScriptIsExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not use Unix mode bits")
	}

	path, err := writeScript("run.sh", "#!/bin/sh\n")
	if err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Fatalf("script is not executable: %v", info.Mode())
	}
}
