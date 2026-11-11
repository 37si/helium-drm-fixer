package proc

import (
	"runtime"
	"testing"
)

func TestHeliumSeesTheChromiumBinaryInAPerUserInstall(t *testing.T) {
	const exe = `C:\Users\ddd\AppData\Local\imput\Helium\Application\chrome.exe`

	snap := Snapshot{Procs: []Process{
		{PID: 100, Parent: 1, Name: "chrome.exe", Exe: exe},
		{PID: 101, Parent: 100, Name: "chrome.exe", Exe: exe},
		{PID: 102, Parent: 100, Name: "chrome.exe", Exe: exe},
	}}

	status := snap.Helium("")
	if !status.Running {
		t.Fatal("a per-user Helium install was not seen as running")
	}
	if len(status.PIDs) != 3 {
		t.Fatalf("pids = %v", status.PIDs)
	}
	if got := status.Describe(); got != exe {
		t.Fatalf("describe = %q", got)
	}
}

func TestHeliumIgnoresGoogleChrome(t *testing.T) {
	snap := Snapshot{Procs: []Process{
		{PID: 200, Parent: 1, Name: "chrome.exe", Exe: `C:\Program Files\Google\Chrome\Application\chrome.exe`},
		{PID: 201, Parent: 200, Name: "chrome.exe", Exe: `C:\Program Files\Google\Chrome\Application\chrome.exe`},
	}}

	if status := snap.Helium(""); status.Running {
		t.Fatalf("Google Chrome was reported as Helium: %+v", status)
	}
}

func TestHeliumMatchesTheRootWhenThePathIsNotNamedHelium(t *testing.T) {
	const exe = `D:\Apps\Browser\Application\chrome.exe`

	snap := Snapshot{Procs: []Process{{PID: 300, Parent: 1, Name: "chrome.exe", Exe: exe}}}

	if status := snap.Helium(`D:\Apps\Browser`); !status.Running || !status.Matches {
		t.Fatalf("the install root did not match: %+v", status)
	}

	if status := snap.Helium(`D:\Apps\Chromium`); status.Matches {
		t.Fatalf("a different install matched: %+v", status)
	}
}

func TestHeliumSeesAStagedUpdate(t *testing.T) {
	snap := Snapshot{Procs: []Process{{
		PID:  400,
		Name: "new_chrome.exe",
		Exe:  `C:\Users\ddd\AppData\Local\imput\Helium\Application\new_chrome.exe`,
	}}}

	if status := snap.Helium(""); !status.Running || !status.Browser {
		t.Fatalf("a staged update was not seen as the browser: %+v", status)
	}
}

func TestBrowserIsTheProcessWithoutAHeliumParent(t *testing.T) {
	const exe = `C:\Users\ddd\AppData\Local\imput\Helium\Application\chrome.exe`

	snap := Snapshot{Procs: []Process{
		{PID: 500, Parent: 4242, Name: "chrome.exe", Exe: exe},
		{PID: 501, Parent: 500, Name: "chrome.exe", Exe: exe},
		{PID: 502, Parent: 500, Name: "chrome.exe", Exe: exe},
	}}

	status := snap.Helium("")
	if !status.Running {
		t.Fatal("not running")
	}
	if !status.Browser {
		t.Fatal("the process started by Explorer is the browser")
	}

	orphan := Snapshot{Procs: []Process{
		{PID: 501, Parent: 500, Name: "chrome.exe", Exe: exe},
	}}

	if status := orphan.Helium(""); !status.Running {
		t.Fatalf("an orphaned helper should still count as running: %+v", status)
	}
}

func TestHeliumMatchesTheRootRegardlessOfCase(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("paths are case sensitive outside Windows")
	}

	snap := Snapshot{Procs: []Process{{
		PID:    600,
		Parent: 1,
		Name:   "chrome.exe",
		Exe:    `C:\USERS\DDD\AppData\Local\imput\Helium\Application\chrome.exe`,
	}}}

	if status := snap.Helium(`C:\Users\ddd\AppData\Local\imput\Helium`); !status.Matches {
		t.Fatalf("case difference defeated the root match: %+v", status)
	}
}

func TestHeliumNeedsAPathForAChromiumName(t *testing.T) {
	snap := Snapshot{Procs: []Process{{PID: 700, Name: "chrome.exe"}}}

	if status := snap.Helium(""); status.Running {
		t.Fatalf("an unreadable chrome.exe was reported as Helium: %+v", status)
	}
}
