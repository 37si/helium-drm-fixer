package proc

import (
	"os"
	"testing"
)

func TestRunningExeSeesThisProcess(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}

	if !RunningExe(executable) {
		t.Fatalf("%s should be running", executable)
	}

	if RunningExe("/nonexistent/helium") {
		t.Fatal("a missing binary should not be running")
	}

	if RunningExe("") {
		t.Fatal("an empty path should not be running")
	}
}

func TestRunningMatchesNames(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}

	if len(Running(base(executable))) == 0 {
		t.Fatalf("%s should be listed", base(executable))
	}

	if got := Running("definitely-not-running-xyz"); len(got) != 0 {
		t.Fatalf("unexpected processes: %v", got)
	}
}

func TestNormalize(t *testing.T) {
	for _, name := range []string{"Helium.exe", "HELIUM", " helium ", "helium.EXE"} {
		if got := normalize(name); got != "helium" {
			t.Fatalf("normalize(%q) = %q", name, got)
		}
	}

	if !matches("Helium.exe", []string{"helium"}) {
		t.Fatal("Helium.exe should match helium")
	}

	if matches("helium-browser", []string{"helium"}) {
		t.Fatal("helium-browser should not match helium")
	}
}

func TestBase(t *testing.T) {
	cases := map[string]string{
		"/usr/bin/helium":                    "helium",
		`C:\Program Files\Helium\Helium.exe`: "Helium.exe",
		"helium":                             "helium",
		"":                                   "",
	}

	for path, want := range cases {
		if got := base(path); got != want {
			t.Fatalf("base(%q) = %q, want %q", path, got, want)
		}
	}
}
