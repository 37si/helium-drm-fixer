package proc

import (
	"runtime"
	"testing"
)

func TestHeliumMatchesNameRegardlessOfCase(t *testing.T) {
	for _, name := range []string{"Helium", "helium", "HELIUM", "Helium.exe"} {
		snap := Snapshot{Procs: []Process{{PID: 42, Name: name, Exe: "/Applications/Helium.app/Contents/MacOS/Helium"}}}

		if status := snap.Helium(""); !status.Running {
			t.Fatalf("%q was not seen as Helium", name)
		}
	}
}

func TestHeliumMatchesPackagedNames(t *testing.T) {
	for _, name := range []string{"helium-bin", "helium-browser", "helium_crashpad_handler", "helium-0.10.7.1-x86_64"} {
		snap := Snapshot{Procs: []Process{{PID: 7, Name: name}}}

		if status := snap.Helium(""); !status.Running {
			t.Fatalf("%q was not seen as Helium", name)
		}
	}
}

func TestHeliumIgnoresOtherPrograms(t *testing.T) {
	snap := Snapshot{Procs: []Process{
		{PID: 1, Name: "systemd"},
		{PID: 2, Name: "chrome"},
		{PID: 3, Name: "heliumtray"},
	}}

	if status := snap.Helium(""); status.Running {
		t.Fatalf("unexpected match: %+v", status)
	}
}

func TestHeliumMatchesTheInstallRoot(t *testing.T) {
	snap := Snapshot{Procs: []Process{{PID: 9, Name: "helium", Exe: "/opt/helium/helium"}}}

	if status := snap.Helium("/opt/helium"); !status.Matches {
		t.Fatal("a process from the install root should match it")
	}

	if status := snap.Helium("/opt/helium-browser"); status.Matches {
		t.Fatal("a sibling directory should not match")
	}

	if status := snap.Helium("/opt/helium-browser-bin"); status.Matches {
		t.Fatal("a prefix sibling should not match")
	}
}

func TestHeliumMatchesAnAppImageMount(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("AppImage paths only exist on Linux")
	}

	snap := Snapshot{Procs: []Process{{
		PID:  11,
		Name: "helium",
		Exe:  "/tmp/.mount_heliumAbC123/opt/helium/helium",
	}}}

	if status := snap.Helium("/opt/helium"); !status.Matches {
		t.Fatalf("an AppImage mount should match its install root: %+v", status)
	}
}

func TestHeliumDoesNotMatchASiblingUnderAnAppImage(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("AppImage paths only exist on Linux")
	}

	snap := Snapshot{Procs: []Process{{
		PID:  12,
		Name: "helium",
		Exe:  "/tmp/.mount_heliumAbC123/opt/helium-old/helium",
	}}}

	if status := snap.Helium("/opt/helium"); status.Matches {
		t.Fatalf("a sibling install should not match: %+v", status)
	}
}

func TestRunningExeSeesARenamedProcess(t *testing.T) {
	snap := Snapshot{Procs: []Process{{PID: 3, Name: "browser", Exe: "/opt/helium/helium"}}}

	if !snap.RunningExe("/opt/helium/helium") {
		t.Fatal("the executable should be found by path")
	}
}

func TestHeliumTellsTheBrowserFromItsHelpers(t *testing.T) {
	helpers := Snapshot{Procs: []Process{{
		PID:  12,
		Name: "helium",
		Exe:  "/opt/helium/helium",
		Args: []string{"/opt/helium/helium", "--type=renderer"},
	}}}

	if status := helpers.Helium("/opt/helium"); status.Browser {
		t.Fatal("a renderer is not the browser process")
	}

	browser := Snapshot{Procs: []Process{{
		PID:  13,
		Name: "helium",
		Exe:  "/opt/helium/helium",
		Args: []string{"/opt/helium/helium", "--profile-directory=Default"},
	}}}

	if status := browser.Helium("/opt/helium"); !status.Browser {
		t.Fatal("a process without --type= is the browser process")
	}
}

func TestStatusDescribesTheRunningCopy(t *testing.T) {
	snap := Snapshot{Procs: []Process{
		{PID: 21, Name: "helium", Exe: "/opt/helium/helium"},
		{PID: 22, Name: "helium", Exe: "/opt/helium/helium", Args: []string{"helium", "--type=gpu-process"}},
	}}

	status := snap.Helium("/opt/helium")
	if len(status.PIDs) != 2 {
		t.Fatalf("pids = %v", status.PIDs)
	}
	if got := status.Describe(); got != "/opt/helium/helium" {
		t.Fatalf("describe = %q", got)
	}
	if got := status.Names(); len(got) != 1 || got[0] != "helium" {
		t.Fatalf("names = %v", got)
	}
}
