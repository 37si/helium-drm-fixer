package tui

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/log/v2"
	"github.com/charmbracelet/x/ansi"

	"hdf/internal/browser"
	"hdf/internal/doctor"
	"hdf/internal/fixer"
	"hdf/internal/helium"
	"hdf/internal/state"
	"hdf/internal/status"
	"hdf/internal/watch"
)

func testModel(t *testing.T, opts Options) *Model {
	t.Helper()

	if opts.Version == "" {
		opts.Version = "test"
	}

	m := New(t.Context(), opts, log.New(io.Discard))
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 32})

	return model.(*Model)
}

func chromeDetection(available bool) browser.Detection {
	detection := browser.Detection{
		Browser:    browser.Browser{ID: browser.Chrome, Name: "Google Chrome", Priority: 10},
		Executable: "/usr/bin/google-chrome",
	}

	if available {
		detection.CDM = &browser.Location{
			Path:    "/opt/google/chrome/WidevineCdm",
			Origin:  browser.OriginBundled,
			Version: "4.10.2934.0",
		}
	}

	return detection
}

func samplePlan() fixer.Plan {
	return fixer.Plan{
		Source: fixer.Source{Name: "Google Chrome", Version: "4.10.2934.0", Path: "/opt/google/chrome/WidevineCdm"},
		Target: "/root/.config/net.imput.helium/WidevineCdm/4.10.2934.0",
		Helium: helium.Install{Root: "/opt/helium-browser-bin", Dir: "/opt/helium-browser-bin"},
	}
}

func sampleReport() status.Report {
	return status.Report{
		Found:  true,
		Helium: helium.Install{Root: "/opt/helium-browser-bin", Dir: "/opt/helium-browser-bin"},
		CDM: &status.Installed{
			Path:    "/root/.config/net.imput.helium/WidevineCdm/4.10.2934.0",
			Version: "4.10.2934.0",
			Managed: true,
			Source:  "Brave",
			At:      time.Now().Add(-2 * time.Hour),
			Size:    18 << 20,
			Files:   6,
		},
		Newest: status.Candidate{
			Available: true,
			Name:      "Brave",
			ID:        browser.Brave,
			Version:   "4.10.3112.0",
			Path:      "/opt/brave/WidevineCdm",
		},
		Backups: 1,
		Checked: time.Now(),
	}
}

func sampleChecks() []doctor.Check {
	return []doctor.Check{
		{Name: "Helium", Level: doctor.LevelOK, Detail: "/opt/helium-browser-bin"},
		{Name: "WidevineCdm", Level: doctor.LevelOK, Detail: "4.10.2934.0 · 18.0 MB · 6 files"},
		{Name: "origin", Level: doctor.LevelOK, Detail: "installed by hdf from Brave, 2 hours ago"},
		{Name: "version", Level: doctor.LevelWarn, Detail: "4.10.2934.0 installed, 4.10.3112.0 available from Brave", Hint: "run hdf and pick Brave"},
		{Name: "write access", Level: doctor.LevelFail, Detail: "permission denied", Hint: "run as an administrator"},
		{Name: "disk", Level: doctor.LevelInfo, Detail: "free space could not be read"},
	}
}

func sampleEvents() []watch.Event {
	return []watch.Event{
		{At: time.Now().Add(-time.Minute), Kind: watch.KindArmed, Message: "watching every 30s"},
		{At: time.Now(), Kind: watch.KindApplied, Message: "installed 4.10.3112.0 from Brave", Version: "4.10.3112.0"},
		{At: time.Now(), Kind: watch.KindError, Message: "no source: no WidevineCdm source found"},
	}
}

func TestScanStreamsRows(t *testing.T) {
	m := testModel(t, Options{})

	model, _ := m.Update(scanMsg{detection: chromeDetection(true)})
	m = model.(*Model)

	if m.scanned != 1 {
		t.Fatalf("scanned = %d, want 1", m.scanned)
	}
	if _, ok := m.seen[browser.Chrome]; !ok {
		t.Fatal("chrome was not recorded")
	}

	out := m.render()
	if !strings.Contains(out, "4.10.2934.0") {
		t.Fatalf("scan view does not show the found version:\n%s", out)
	}
}

func TestScanPopulatesList(t *testing.T) {
	m := testModel(t, Options{})

	model, _ := m.Update(scanDoneMsg{detections: []browser.Detection{chromeDetection(true)}})
	m = model.(*Model)

	if m.screen != screenPicking {
		t.Fatalf("screen = %v, want picking", m.screen)
	}
	if len(m.list.Items()) != 2 {
		t.Fatalf("items = %d, want the fresh download and Chrome", len(m.list.Items()))
	}

	m.list.Select(1)

	out := m.render()
	for _, want := range []string{"Google Chrome", "4.10.2934.0", "/opt/google/chrome/", "details"} {
		if !strings.Contains(out, want) {
			t.Fatalf("view is missing %q:\n%s", want, out)
		}
	}
}

func TestPickerOffersTheFreshDownloadWhenNothingIsInstalled(t *testing.T) {
	m := testModel(t, Options{})

	model, _ := m.Update(scanDoneMsg{detections: []browser.Detection{chromeDetection(false)}})
	m = model.(*Model)

	if m.screen != screenPicking {
		t.Fatalf("screen = %v, want picking", m.screen)
	}

	items := m.list.Items()
	if len(items) != 2 {
		t.Fatalf("items = %d, want the fresh download and Chrome", len(items))
	}

	first, ok := items[0].(sourceItem)
	if !ok || !first.fresh {
		t.Fatalf("the first row should be the fresh download: %#v", items[0])
	}

	second, ok := items[1].(sourceItem)
	if !ok || second.usable() {
		t.Fatalf("Chrome has no CDM and should not be usable: %#v", items[1])
	}
}

func TestHelpScreen(t *testing.T) {
	m := testModel(t, Options{})
	model, _ := m.Update(scanDoneMsg{detections: []browser.Detection{chromeDetection(true)}})
	m = model.(*Model)

	model, _ = m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	m = model.(*Model)

	if m.screen != screenHelp {
		t.Fatalf("screen = %v, want help", m.screen)
	}
	if out := m.render(); !strings.Contains(out, "WidevineCdm") {
		t.Fatalf("help view is empty:\n%s", out)
	}

	model, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = model.(*Model)

	if m.screen != screenPicking {
		t.Fatalf("screen = %v, want picking after closing help", m.screen)
	}
}

func TestEnterOpensReview(t *testing.T) {
	m := testModel(t, Options{})
	model, _ := m.Update(scanDoneMsg{detections: []browser.Detection{chromeDetection(true)}})
	m = model.(*Model)

	m.list.Select(1)

	model, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = model.(*Model)

	if m.screen != screenReview {
		t.Fatalf("screen = %v, want review", m.screen)
	}
	if cmd == nil {
		t.Fatal("expected the plan to be resolved")
	}
	if m.opts.Fixer.Browser != browser.Chrome {
		t.Fatalf("browser = %q", m.opts.Fixer.Browser)
	}

	model, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = model.(*Model)

	if m.screen != screenPicking || m.form != nil {
		t.Fatalf("screen = %v, form = %v", m.screen, m.form)
	}
}

func TestReviewShowsPlanAndForm(t *testing.T) {
	m := testModel(t, Options{})

	model, cmd := m.Update(planMsg{plan: samplePlan()})
	m = model.(*Model)

	if m.screen != screenReview {
		t.Fatalf("screen = %v, want review", m.screen)
	}
	if m.form == nil || cmd == nil {
		t.Fatal("expected the form to be built")
	}

	out := m.render()
	for _, want := range []string{"plan", "source", "Google Chrome", "target", "/root/.config/net.imput.helium/", "Helium directory"} {
		if !strings.Contains(out, want) {
			t.Fatalf("review view is missing %q:\n%s", want, out)
		}
	}
}

func TestPlanStartsApply(t *testing.T) {
	m := testModel(t, Options{})

	plan := samplePlan()
	plan.Source.Path = filepath.Join(t.TempDir(), "missing")

	model, _ := m.Update(planMsg{plan: plan, apply: true})
	m = model.(*Model)

	if m.screen != screenApplying {
		t.Fatalf("screen = %v, want applying", m.screen)
	}
}

func TestPlanErrorFails(t *testing.T) {
	m := testModel(t, Options{})
	model, _ := m.Update(planMsg{err: fixer.ErrNoSource})
	m = model.(*Model)

	if m.screen != screenFailed {
		t.Fatalf("screen = %v, want failed", m.screen)
	}
}

func TestDryRunSkipsApply(t *testing.T) {
	m := testModel(t, Options{DryRun: true})

	plan := samplePlan()
	model, _ := m.Update(planMsg{plan: plan, apply: true})
	m = model.(*Model)

	if m.screen != screenDone {
		t.Fatalf("screen = %v, want done", m.screen)
	}

	out := m.render()
	if !strings.Contains(out, "dry run") {
		t.Fatalf("view does not mention the dry run:\n%s", out)
	}
	if !strings.Contains(m.render(), "/root/.config/net.imput.helium/") {
		t.Fatalf("view is missing the target:\n%s", out)
	}
}

func TestApplyResultShowsSummary(t *testing.T) {
	m := testModel(t, Options{})
	m.plan = samplePlan()

	model, _ := m.Update(applyMsg{result: fixer.Result{Target: m.plan.Target}})
	m = model.(*Model)

	if m.screen != screenDone {
		t.Fatalf("screen = %v, want done", m.screen)
	}

	out := ansi.Strip(m.render())
	for _, want := range []string{"WidevineCdm installed", "Google Chrome", "/root/.config/net.imput.helium/", "Restart Helium"} {
		if !strings.Contains(out, want) {
			t.Fatalf("view is missing %q:\n%s", want, out)
		}
	}
}

func TestStepUpdatesProgress(t *testing.T) {
	m := testModel(t, Options{})

	model, _ := m.Update(stepMsg{step: "copying WidevineCdm", fraction: 0.6})
	m = model.(*Model)

	if m.step != "copying WidevineCdm" {
		t.Fatalf("step = %q", m.step)
	}
	if m.target != 0.6 {
		t.Fatalf("target = %v", m.target)
	}
	if !m.animating {
		t.Fatal("expected the spring to be animating")
	}
}

func TestStepsAccumulate(t *testing.T) {
	m := testModel(t, Options{})

	for _, name := range []string{"copying WidevineCdm", "writing component updater hint"} {
		model, _ := m.Update(stepMsg{step: name, fraction: 0.5})
		m = model.(*Model)
	}

	if len(m.steps) != 2 {
		t.Fatalf("steps = %d, want 2", len(m.steps))
	}
	if m.steps[0].state != stepDone {
		t.Fatalf("first step state = %v, want done", m.steps[0].state)
	}
	if m.steps[1].state != stepActive {
		t.Fatalf("second step state = %v, want active", m.steps[1].state)
	}
}

func TestSpringSettlesOnTarget(t *testing.T) {
	m := testModel(t, Options{})
	m.spring.pos = 0
	m.target = 1
	m.animating = true

	for i := 0; i < 600; i++ {
		model, _ := m.Update(tickMsg{})
		m = model.(*Model)
		if !m.animating {
			break
		}
	}

	if m.spring.pos < 0.99 {
		t.Fatalf("spring did not settle: %v", m.spring.pos)
	}
}

func TestQuitKey(t *testing.T) {
	m := testModel(t, Options{})

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("expected a quit command")
	}

	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", cmd())
	}
}

func TestRescanResetsState(t *testing.T) {
	m := testModel(t, Options{})
	model, _ := m.Update(scanDoneMsg{detections: []browser.Detection{chromeDetection(true)}})
	m = model.(*Model)

	model, _ = m.Update(applyMsg{result: fixer.Result{Target: "/tmp/x"}})
	m = model.(*Model)

	model, cmd := m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	m = model.(*Model)

	if m.screen != screenScanning {
		t.Fatalf("screen = %v, want scanning", m.screen)
	}
	if m.scanned != 0 || len(m.seen) != 0 {
		t.Fatalf("scan state was not cleared: %d scanned, %d seen", m.scanned, len(m.seen))
	}
	if cmd == nil {
		t.Fatal("expected a rescan command")
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Fatalf("truncate = %q", got)
	}
	if got := truncate("0123456789abcdef", 6); got != "…bcdef" {
		t.Fatalf("truncate = %q", got)
	}
}

func TestApplyingViewShowsProgress(t *testing.T) {
	m := testModel(t, Options{})
	m.screen = screenApplying
	m.recordStep("copying WidevineCdm")
	m.spring.pos = 0.42
	m.plan = samplePlan()

	out := m.render()
	for _, want := range []string{"copying WidevineCdm", "42%", "Google Chrome", "/root/.config/net.imput.helium/"} {
		if !strings.Contains(out, want) {
			t.Fatalf("view is missing %q:\n%s", want, out)
		}
	}
}

func TestNarrowTerminalDropsDetailPane(t *testing.T) {
	m := testModel(t, Options{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 58, Height: 20})
	m = model.(*Model)
	model, _ = m.Update(scanDoneMsg{detections: []browser.Detection{chromeDetection(true)}})
	m = model.(*Model)

	if l := m.layout(); !l.stacked {
		t.Fatalf("expected a stacked layout at 58 columns")
	}
	if out := m.render(); strings.Contains(out, "details") {
		t.Fatalf("detail pane should be hidden:\n%s", out)
	}
}

func TestPanelsAreClosed(t *testing.T) {
	m := testModel(t, Options{})
	model, _ := m.Update(scanDoneMsg{detections: []browser.Detection{chromeDetection(true)}})
	m = model.(*Model)

	for _, line := range strings.Split(m.render(), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "│") {
			continue
		}
		if !strings.HasSuffix(strings.TrimRight(line, " "), "│") {
			t.Fatalf("panel row is not closed: %q", line)
		}
	}
}

func TestLayoutFitsWidth(t *testing.T) {
	for _, size := range []struct{ w, h int }{{40, 12}, {60, 20}, {80, 24}, {100, 32}, {140, 40}} {
		m := testModel(t, Options{})
		model, _ := m.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})
		m = model.(*Model)
		model, _ = m.Update(scanDoneMsg{detections: []browser.Detection{chromeDetection(true)}})
		m = model.(*Model)

		for _, screen := range allScreens() {
			m.screen = screen
			if screen == screenHelp {
				m.history = append(m.history[:0], screenPicking)
				m.viewport.SetContent(m.renderHelp())
			}

			for _, line := range strings.Split(m.render(), "\n") {
				if got := lipgloss.Width(line); got > size.w {
					t.Fatalf("%dx%d screen %d: line is %d cells wide: %q", size.w, size.h, screen, got, line)
				}
			}
		}
	}
}

func TestStatusScreenShowsInstall(t *testing.T) {
	m := testModel(t, Options{})
	model, _ := m.Update(statusMsg{report: sampleReport()})
	m = model.(*Model)
	m.screen = screenStatus

	out := m.render()
	for _, want := range []string{"installed", "4.10.2934.0", "Brave", "update available", "backups"} {
		if !strings.Contains(out, want) {
			t.Fatalf("status view is missing %q:\n%s", want, out)
		}
	}
}

func TestStatusScreenWithoutHelium(t *testing.T) {
	m := testModel(t, Options{})
	model, _ := m.Update(statusMsg{report: status.Report{}})
	m = model.(*Model)
	m.screen = screenStatus

	if out := m.render(); !strings.Contains(out, "was not found") {
		t.Fatalf("status view does not explain the miss:\n%s", out)
	}
}

func TestDoctorScreenListsChecks(t *testing.T) {
	m := testModel(t, Options{})
	model, _ := m.Update(doctorMsg{checks: sampleChecks()})
	m = model.(*Model)
	m.screen = screenDoctor

	out := m.render()
	for _, want := range []string{"doctor", "WidevineCdm", "permission denied", "run as an administrator", "checks"} {
		if !strings.Contains(out, want) {
			t.Fatalf("doctor view is missing %q:\n%s", want, out)
		}
	}
}

func TestDoctorScrolls(t *testing.T) {
	m := testModel(t, Options{})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 14})
	m = model.(*Model)
	model, _ = m.Update(doctorMsg{checks: sampleChecks()})
	m = model.(*Model)
	m.screen = screenDoctor

	model, _ = m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = model.(*Model)

	if m.scroll != 1 {
		t.Fatalf("scroll = %d, want 1", m.scroll)
	}

	model, _ = m.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	m = model.(*Model)

	if m.scroll != 0 {
		t.Fatalf("scroll = %d, want 0", m.scroll)
	}
}

func TestWatchEventsAppend(t *testing.T) {
	m := testModel(t, Options{})
	m.screen = screenWatch

	model, _ := m.Update(watchMsg{event: watch.Event{Kind: watch.KindApplied, Message: "installed 4.10.3112.0 from Brave"}})
	m = model.(*Model)

	if len(m.watchEvents) != 1 {
		t.Fatalf("events = %d, want 1", len(m.watchEvents))
	}
	if out := m.render(); !strings.Contains(out, "installed 4.10.3112.0") {
		t.Fatalf("watch view does not show the event:\n%s", out)
	}
}

func TestWatchPauses(t *testing.T) {
	m := testModel(t, Options{})
	m.screen = screenWatch

	model, _ := m.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	m = model.(*Model)

	if !m.watchPaused {
		t.Fatal("space did not pause the watcher")
	}
	if out := m.render(); !strings.Contains(out, "paused") {
		t.Fatalf("watch view does not show the pause:\n%s", out)
	}

	model, _ = m.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	m = model.(*Model)

	if m.watchPaused {
		t.Fatal("space did not resume the watcher")
	}
}

func TestWatchIntervalChanges(t *testing.T) {
	m := testModel(t, Options{})
	m.screen = screenWatch
	before := m.watchInterval

	model, _ := m.Update(tea.KeyPressMsg{Code: '+', Text: "+"})
	m = model.(*Model)

	if m.watchInterval <= before {
		t.Fatalf("interval = %v, want more than %v", m.watchInterval, before)
	}

	model, _ = m.Update(tea.KeyPressMsg{Code: '-', Text: "-"})
	m = model.(*Model)

	if m.watchInterval != before {
		t.Fatalf("interval = %v, want %v", m.watchInterval, before)
	}
}

func TestStatusOpensFromPicker(t *testing.T) {
	m := testModel(t, Options{})
	model, _ := m.Update(scanDoneMsg{detections: []browser.Detection{chromeDetection(true)}})
	m = model.(*Model)

	model, cmd := m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	m = model.(*Model)

	if m.screen != screenStatus {
		t.Fatalf("screen = %v, want status", m.screen)
	}
	if cmd == nil {
		t.Fatal("expected the status to be read")
	}

	model, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = model.(*Model)

	if m.screen != screenPicking {
		t.Fatalf("screen = %v, want picking after closing the status", m.screen)
	}
}

func TestUndoWithoutBackupExplains(t *testing.T) {
	m := testModel(t, Options{})

	if cmd := m.undo(); cmd != nil {
		t.Fatal("expected no command without a backup")
	}
	if !strings.Contains(m.notice, "no backup") {
		t.Fatalf("notice = %q", m.notice)
	}
}

func TestUndoAfterADryRunDoesNothing(t *testing.T) {
	m := testModel(t, Options{DryRun: true})
	m.screen = screenDone

	if cmd := m.undo(); cmd != nil {
		t.Fatal("a dry run should not start a restore")
	}
	if !strings.Contains(m.notice, "dry run") {
		t.Fatalf("notice = %q", m.notice)
	}
}

func TestCtrlCWaitsForAnApplyToFinish(t *testing.T) {
	m := testModel(t, Options{})
	m.screen = screenApplying

	key := tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}

	model, cmd := m.Update(key)
	m = model.(*Model)

	if cmd != nil {
		t.Fatal("the first ctrl+c should not quit during an apply")
	}
	if !m.quitAfterApply {
		t.Fatal("the apply was not marked for quitting")
	}
	if m.notice == "" {
		t.Fatal("no notice was shown")
	}

	model, cmd = m.Update(key)
	m = model.(*Model)

	if cmd == nil {
		t.Fatal("the second ctrl+c should quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", cmd())
	}

	m.screen = screenApplying
	m.quitAfterApply = true

	model, cmd = m.Update(applyMsg{result: fixer.Result{Target: "/tmp/cdm"}})
	m = model.(*Model)

	if cmd == nil {
		t.Fatal("the finished apply should quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", cmd())
	}
}

func TestWatchIntervalComesFromTheCLI(t *testing.T) {
	if got := New(t.Context(), Options{Interval: 2 * time.Minute}, log.New(io.Discard)).watchInterval; got != 2*time.Minute {
		t.Fatalf("interval = %v", got)
	}
	if got := New(t.Context(), Options{Interval: time.Second}, log.New(io.Discard)).watchInterval; got != watch.MinInterval {
		t.Fatalf("interval = %v, want %v", got, watch.MinInterval)
	}
	if got := New(t.Context(), Options{}, log.New(io.Discard)).watchInterval; got != 30*time.Second {
		t.Fatalf("default interval = %v", got)
	}
}

func TestWatchLogScrolls(t *testing.T) {
	m := testModel(t, Options{})
	m.screen = screenWatch
	m.watchEvents = make([]watch.Event, 40)

	model, _ := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	m = model.(*Model)

	if m.watchScroll != 1 {
		t.Fatalf("watch scroll = %d, want 1", m.watchScroll)
	}

	model, _ = m.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	m = model.(*Model)

	if m.watchScroll != 0 {
		t.Fatalf("watch scroll = %d, want 0", m.watchScroll)
	}
}

func TestDoneHintsFollowTheResult(t *testing.T) {
	dry := testModel(t, Options{DryRun: true})
	dry.screen = screenDone
	if hintsContain(dry.keyHints(), "v") || hintsContain(dry.keyHints(), "u") {
		t.Fatalf("dry-run hints = %v", dry.keyHints())
	}

	installed := testModel(t, Options{})
	installed.screen = screenDone
	installed.result.Backup = "backup-id"
	if !hintsContain(installed.keyHints(), "v") || !hintsContain(installed.keyHints(), "u") {
		t.Fatalf("install hints = %v", installed.keyHints())
	}
}

func hintsContain(hints [][2]string, key string) bool {
	for _, hint := range hints {
		if hint[0] == key {
			return true
		}
	}

	return false
}

func TestClearBackupsWithoutSnapshotsExplains(t *testing.T) {
	m := testModel(t, Options{Fixer: fixer.Options{BackupDir: t.TempDir(), Backups: 5}})
	m.screen = screenStatus

	model, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	m = model.(*Model)

	if cmd != nil {
		t.Fatal("no command should start without backups")
	}
	if m.screen != screenStatus {
		t.Fatalf("screen = %v, want status", m.screen)
	}
	if !strings.Contains(m.notice, "no backups") {
		t.Fatalf("notice = %q", m.notice)
	}
	if out := m.render(); !strings.Contains(out, "no backups") {
		t.Fatalf("status view does not show the notice:\n%s", out)
	}
}

func TestClearBackupsNeedsTwoConfirmations(t *testing.T) {
	dir := t.TempDir()
	id := "20260101-000000.000"
	backupDir := filepath.Join(dir, id)
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "meta.json"), []byte(`{"id":"`+id+`","bytes":10}`), 0o644); err != nil {
		t.Fatal(err)
	}

	m := testModel(t, Options{Fixer: fixer.Options{BackupDir: dir, Backups: 5}})
	m.screen = screenStatus
	m.openClear()

	if m.screen != screenClear || m.clearCount != 1 {
		t.Fatalf("screen = %v, count = %d", m.screen, m.clearCount)
	}

	model, _ := m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	m = model.(*Model)

	if m.clearStage != 1 {
		t.Fatalf("stage = %d, want 1", m.clearStage)
	}
	if _, err := os.Stat(backupDir); err != nil {
		t.Fatal("the first confirmation deleted the backup")
	}

	model, cmd := m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	m = model.(*Model)

	if !m.clearBusy || cmd == nil {
		t.Fatal("the second confirmation did not start clearing")
	}

	model, _ = m.Update(cmd())
	m = model.(*Model)

	if m.screen != screenStatus {
		t.Fatalf("screen = %v, want status", m.screen)
	}
	if _, err := os.Stat(backupDir); !os.IsNotExist(err) {
		t.Fatalf("backup was not removed: %v", err)
	}
	if !strings.Contains(m.notice, "removed 1") {
		t.Fatalf("notice = %q", m.notice)
	}
}

func TestHelpPausesTheWatcher(t *testing.T) {
	m := testModel(t, Options{})
	m.screen = screenWatch

	stopped := false
	m.watchCancel = func() { stopped = true }

	model, _ := m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	m = model.(*Model)

	if !stopped {
		t.Fatal("opening help left the watcher running")
	}
	if !m.watchPaused {
		t.Fatal("the watcher was not marked paused")
	}
	if m.screen != screenHelp {
		t.Fatalf("screen = %v, want help", m.screen)
	}
}

func TestVerifyConfirmationIsRecorded(t *testing.T) {
	dir := t.TempDir()
	m := testModel(t, Options{Fixer: fixer.Options{StateDir: dir}})
	m.screen = screenDone
	m.result.Target = "/tmp/widevine"
	m.verify = verifyAsking

	model, _ := m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	m = model.(*Model)

	if m.verify != verifyConfirmed {
		t.Fatalf("verify = %v, want confirmed", m.verify)
	}
	if out := m.render(); !strings.Contains(out, "playback confirmed") {
		t.Fatalf("result view does not show the confirmation:\n%s", out)
	}
}

func TestVerifyDeclinedStaysQuiet(t *testing.T) {
	m := testModel(t, Options{})
	m.screen = screenDone
	m.verify = verifyAsking

	model, _ := m.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	m = model.(*Model)

	if m.verify != verifyDeclined {
		t.Fatalf("verify = %v, want declined", m.verify)
	}
}

func braveDetection() browser.Detection {
	return browser.Detection{
		Browser:    browser.Browser{ID: browser.Brave, Name: "Brave", Priority: 30},
		Executable: "/usr/bin/brave-browser",
		CDM: &browser.Location{
			Path:    "/opt/brave/WidevineCdm",
			Origin:  browser.OriginBundled,
			Version: "4.10.3112.0",
		},
	}
}

func TestPickerPreselectsLastSource(t *testing.T) {
	dir := t.TempDir()
	state.Open(dir).Append(state.Record{
		Target:   "/tmp/cdm",
		Source:   "Brave",
		SourceID: string(browser.Brave),
		Tool:     "test",
		Method:   state.MethodCopy,
	})

	m := testModel(t, Options{Fixer: fixer.Options{StateDir: dir}})
	model, _ := m.Update(scanDoneMsg{detections: []browser.Detection{chromeDetection(true), braveDetection()}})
	m = model.(*Model)

	item, ok := m.list.SelectedItem().(sourceItem)
	if !ok || item.detection.ID != browser.Brave {
		t.Fatalf("selected %v, want Brave", m.list.SelectedItem())
	}
}

func TestExplicitBrowserBeatsTheLastSource(t *testing.T) {
	dir := t.TempDir()
	state.Open(dir).Append(state.Record{
		Target:   "/tmp/cdm",
		SourceID: string(browser.Brave),
		Tool:     "test",
		Method:   state.MethodCopy,
	})

	m := testModel(t, Options{Fixer: fixer.Options{StateDir: dir, Browser: browser.Chrome}})
	model, _ := m.Update(scanDoneMsg{detections: []browser.Detection{chromeDetection(true), braveDetection()}})
	m = model.(*Model)

	item, _ := m.list.SelectedItem().(sourceItem)
	if item.detection.ID != browser.Chrome {
		t.Fatalf("selected %v, want the flag value", item.detection.ID)
	}
}

func TestDetailsMentionTheLastUse(t *testing.T) {
	dir := t.TempDir()
	state.Open(dir).Append(state.Record{
		Target:      "/tmp/cdm",
		SourceID:    string(browser.Brave),
		Tool:        "test",
		Method:      state.MethodCopy,
		InstalledAt: time.Now().Add(-3 * time.Hour),
	})

	m := testModel(t, Options{Fixer: fixer.Options{StateDir: dir}})
	model, _ := m.Update(scanDoneMsg{detections: []browser.Detection{braveDetection()}})
	m = model.(*Model)

	if out := m.render(); !strings.Contains(out, "used 3 hours ago") {
		t.Fatalf("details do not mention the last use:\n%s", out)
	}
}

func escKeys() map[string]tea.KeyPressMsg {
	return map[string]tea.KeyPressMsg{
		"bare esc":     {Code: tea.KeyEscape},
		"control char": {Code: 0x1b, Text: "\x1b"},
		"ctrl+[":       {Code: '[', Mod: tea.ModCtrl},
	}
}

func TestEscapeGoesBackInEveryShape(t *testing.T) {
	for name, key := range escKeys() {
		m := testModel(t, Options{})
		model, _ := m.Update(scanDoneMsg{detections: []browser.Detection{chromeDetection(true)}})
		m = model.(*Model)

		model, _ = m.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
		m = model.(*Model)

		if m.screen != screenDoctor {
			t.Fatalf("%s: screen = %v, want doctor", name, m.screen)
		}

		model, _ = m.Update(key)
		m = model.(*Model)

		if m.screen != screenPicking {
			t.Fatalf("%s: screen = %v, want picking", name, m.screen)
		}
	}
}

func TestBackWalksTheScreensVisited(t *testing.T) {
	m := testModel(t, Options{})
	model, _ := m.Update(scanDoneMsg{detections: []browser.Detection{chromeDetection(true)}})
	m = model.(*Model)

	model, _ = m.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	m = model.(*Model)
	model, _ = m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	m = model.(*Model)

	if m.screen != screenStatus {
		t.Fatalf("screen = %v, want status", m.screen)
	}

	model, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = model.(*Model)
	if m.screen != screenDoctor {
		t.Fatalf("screen = %v, want doctor", m.screen)
	}

	model, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = model.(*Model)
	if m.screen != screenPicking {
		t.Fatalf("screen = %v, want picking", m.screen)
	}

	if _, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape}); cmd == nil {
		t.Fatal("escape at the source list should quit")
	}
}

func TestEscapeLeavesTheResultScreen(t *testing.T) {
	m := testModel(t, Options{})
	m.plan = samplePlan()

	model, _ := m.Update(applyMsg{result: fixer.Result{Target: m.plan.Target}})
	m = model.(*Model)

	if m.screen != screenDone {
		t.Fatalf("screen = %v, want done", m.screen)
	}

	model, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = model.(*Model)

	if m.screen != screenPicking {
		t.Fatalf("screen = %v, want picking", m.screen)
	}
	if m.planReady {
		t.Fatal("the finished plan should have been dropped")
	}
}

func TestFreshDownloadIsTheFirstRow(t *testing.T) {
	m := testModel(t, Options{Fixer: fixer.Options{}})
	model, _ := m.Update(scanDoneMsg{detections: []browser.Detection{chromeDetection(true)}})
	m = model.(*Model)

	item, ok := m.list.SelectedItem().(sourceItem)
	if !ok || !item.fresh {
		t.Fatalf("first row is not the fresh download: %#v", m.list.SelectedItem())
	}

	model, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = model.(*Model)

	if cmd == nil {
		t.Fatal("expected the plan to be resolved")
	}
	if !m.opts.Fixer.Fresh {
		t.Fatal("the fresh flag was not set")
	}
	if m.opts.Fixer.Browser != "" {
		t.Fatalf("browser = %q, want none", m.opts.Fixer.Browser)
	}
}

func TestFreshDownloadExplainsItself(t *testing.T) {
	m := testModel(t, Options{Fixer: fixer.Options{}})
	model, _ := m.Update(scanDoneMsg{detections: []browser.Detection{chromeDetection(true)}})
	m = model.(*Model)

	out := ansi.Strip(m.render())
	for _, want := range []string{"Fresh download", "aus5.mozilla.org", "SHA-512", "fallback"} {
		if !strings.Contains(out, want) {
			t.Fatalf("details are missing %q:\n%s", want, out)
		}
	}
}

func TestAnimationClockAdvances(t *testing.T) {
	m := testModel(t, Options{})

	if !m.anim {
		t.Fatal("animation should be on by default")
	}

	if m.phase() != 0 {
		t.Fatalf("phase = %v before any tick", m.phase())
	}

	model, cmd := m.Update(tickMsg{})
	m = model.(*Model)

	if cmd == nil {
		t.Fatal("the clock should keep ticking")
	}
	if m.phase() <= 0 {
		t.Fatalf("phase = %v after a tick", m.phase())
	}

	still := testModel(t, Options{NoAnim: true})
	if still.anim || still.phase() != 0 {
		t.Fatal("--no-anim should freeze the clock")
	}
	if cmd := still.animate(); cmd != nil {
		t.Fatal("--no-anim should not schedule ticks")
	}
}

func TestPickerSaysWhatHeliumHas(t *testing.T) {
	m := testModel(t, Options{})
	model, _ := m.Update(scanDoneMsg{
		detections: []browser.Detection{chromeDetection(true)},
		helium:     "/opt/helium/Application/resources",
		held:       "4.10.1000.0",
	})
	m = model.(*Model)

	m.list.Select(1)

	out := ansi.Strip(m.render())
	for _, want := range []string{"/opt/helium/Application/resources", "4.10.1000.0"} {
		if !strings.Contains(out, want) {
			t.Fatalf("picker does not say what Helium has (%q missing):\n%s", want, out)
		}
	}

	if !strings.Contains(out, "newer") {
		t.Fatalf("a newer copy in the list should be called out:\n%s", out)
	}
}

func TestPickerSaysWhenNothingIsInstalled(t *testing.T) {
	m := testModel(t, Options{})
	model, _ := m.Update(scanDoneMsg{
		detections: []browser.Detection{chromeDetection(true)},
		helium:     "/opt/helium/Application/resources",
	})
	m = model.(*Model)

	out := ansi.Strip(m.render())
	if !strings.Contains(out, "first install") {
		t.Fatalf("an empty install should say so:\n%s", out)
	}
}

func TestHeldBlockStaysOutOfTheWayWhenShort(t *testing.T) {
	m := testModel(t, Options{})
	m.heliumDir = "/opt/helium"
	m.held = "4.10.1000.0"

	if block := m.heldBlock(60, 4); block != nil {
		t.Fatalf("a cramped pane should get no block, got %d lines", len(block))
	}
	if block := m.heldBlock(60, 8); len(block) == 0 {
		t.Fatal("a roomy pane should get the block")
	}
}
