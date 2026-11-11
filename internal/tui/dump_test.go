package tui

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	tea "charm.land/bubbletea/v2"

	"hdf/internal/browser"
	"hdf/internal/doctor"
	"hdf/internal/proc"
	"hdf/internal/status"
)

func TestDumpFrames(t *testing.T) {
	if os.Getenv("HDF_DUMP") == "" {
		t.Skip("set HDF_DUMP=1 to print frames")
	}

	for _, size := range []struct{ w, h int }{{104, 30}, {72, 22}} {
		for _, screen := range []screen{
			screenScanning, screenPicking, screenReview, screenApplying,
			screenDone, screenFailed, screenStatus, screenDoctor, screenWatch, screenClear,
		} {
			m := sizedModel(t, size.w, size.h)
			m.screen = screen
			m.plan = samplePlan()
			m.planReady = true
			m.detections = []browser.Detection{
				{Browser: browser.All[0], Executable: "/usr/bin/google-chrome", CDM: &browser.Location{Path: "/opt/google/chrome/WidevineCdm", Version: "4.10.2934.0", Origin: browser.OriginBundled}},
				{Browser: browser.All[8], Executable: "/usr/bin/brave-browser"},
			}
			m.heliumDir = "/opt/helium"
			m.held = "4.10.2891.0"
			m.running = proc.Status{Running: true, Matches: true, PIDs: []int{4242}, Paths: []string{"/opt/helium/helium"}}
			m.report = status.Report{
				Found:  true,
				Helium: m.plan.Helium,
				CDM: &status.Installed{
					Path: "/root/.config/net.imput.helium/WidevineCdm/4.10.2934.0", Version: "4.10.2934.0",
					Managed: true, Source: "Google Chrome", Size: 19 << 20, Files: 7,
				},
				Newest:  status.Candidate{Available: true, Name: "Google Chrome", Version: "4.10.3112.0"},
				Running: m.running,
				Backups: 2,
			}
			m.reportAt = m.started
			m.checks = []doctor.Check{
				{Name: "helium", Level: doctor.LevelOK, Detail: "/opt/helium"},
				{Name: "installed", Level: doctor.LevelOK, Detail: "4.10.2934.0"},
				{Name: "running", Level: doctor.LevelWarn, Detail: "Helium is running from /opt/helium/helium, a restart is needed after installing", Hint: "quit Helium and start it again after installing"},
				{Name: "disk", Level: doctor.LevelOK, Detail: "41 GB free"},
			}
			m.steps = []applyStep{{name: "checking the source", state: stepDone}, {name: "copying WidevineCdm", state: stepActive}}
			m.spring.pos = 0.42
			m.notice = "restored backup 20261110-101010"

			if screen == screenReview {
				m.form = nil
			}

			frame := m.render()
			fmt.Printf("\n=== %dx%d screen %d ===\n%s\n", size.w, size.h, screen, ansi.Strip(frame))
			fmt.Printf("-- raw top line: %q\n", strings.Split(frame, "\n")[4])
			for i, line := range strings.Split(frame, "\n") {
				if ansi.StringWidth(line) != size.w {
					fmt.Printf("!! row %d is %d cells, want %d\n", i, ansi.StringWidth(line), size.w)
				}
			}
		}
	}

	_ = tea.Quit
}
