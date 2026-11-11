package tui

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"hdf/internal/backup"
	"hdf/internal/browser"
	"hdf/internal/doctor"
	"hdf/internal/fixer"
	"hdf/internal/helium"
	"hdf/internal/proc"
	"hdf/internal/state"
	"hdf/internal/status"
	"hdf/internal/units"
	"hdf/internal/verify"
	"hdf/internal/watch"
)

type verifyState int

const (
	verifyNone verifyState = iota
	verifyAsking
	verifyConfirmed
	verifyDeclined
)

type (
	statusMsg  struct{ report status.Report }
	runningMsg struct{ status proc.Status }
	doctorMsg  struct{ checks []doctor.Check }
	watchMsg   struct {
		event watch.Event
		ch    <-chan tea.Msg
	}
	undoMsg struct {
		id     string
		result fixer.Result
		err    error
	}
	verifyMsg struct {
		stream verify.Stream
		err    error
	}
	clearMsg struct {
		count int
		bytes int64
		err   error
	}
)

func (m *Model) openStatus() tea.Cmd {
	m.show(screenStatus)
	return statusCmd(m.ctx, m.opts.Fixer, m.detections)
}

func statusCmd(ctx context.Context, opts fixer.Options, detections []browser.Detection) tea.Cmd {
	return func() tea.Msg {
		report := status.Inspect(ctx, status.Options{
			HeliumPath: opts.HeliumPath,
			StateDir:   opts.StateDir,
			BackupDir:  opts.BackupDir,
			Backups:    opts.Backups,
		}, detections)

		return statusMsg{report: report}
	}
}

func (m *Model) openDoctor() tea.Cmd {
	m.show(screenDoctor)
	m.checks = nil
	m.scroll = 0
	return m.doctorCmd()
}

func (m *Model) doctorCmd() tea.Cmd {
	ctx, opts, detections, logger := m.ctx, m.opts, m.detections, m.logger

	return func() tea.Msg {
		checks := doctor.Run(ctx, doctor.Options{
			HeliumPath: opts.Fixer.HeliumPath,
			StateDir:   opts.Fixer.StateDir,
			BackupDir:  opts.Fixer.BackupDir,
			Backups:    opts.Fixer.Backups,
			LogPath:    opts.LogPath,
			Version:    opts.Version,
			Repo:       opts.Repo,
			Online:     opts.Online,
		}, detections)

		if logger != nil {
			fails, warns := doctor.Summary(checks)
			logger.Debug("doctor finished", "checks", len(checks), "failed", fails, "warnings", warns)
		}

		return doctorMsg{checks: checks}
	}
}

func (m *Model) openWatch() tea.Cmd {
	m.show(screenWatch)
	m.watchEvents = nil
	m.watchPaused = false
	m.watchScroll = 0

	return m.startWatch()
}

func (m *Model) startWatch() tea.Cmd {
	m.stopWatch()

	ctx, cancel := context.WithCancel(m.ctx)
	m.watchCancel = cancel

	options := m.opts.Fixer
	if options.Browser == "" {
		options.Newest = true
	}

	watcher := watch.New(watch.Options{
		Interval: m.watchInterval,
		Fixer:    options,
		DryRun:   m.dryRun,
	}, m.logger)

	ch := make(chan tea.Msg, 16)
	m.watchCh = ch

	events := watcher.Run(ctx)
	go func() {
		for event := range events {
			ch <- watchMsg{event: event, ch: ch}
		}
		close(ch)
	}()

	return waitFor(ch)
}

func (m *Model) stopWatch() {
	if m.watchCancel != nil {
		m.watchCancel()
		m.watchCancel = nil
	}
}

func (m *Model) onWatchEvent(msg watchMsg) (tea.Model, tea.Cmd) {
	if msg.ch != nil && msg.ch != m.watchCh {
		return m, nil
	}

	m.watchEvents = append(m.watchEvents, msg.event)
	if len(m.watchEvents) > 60 {
		m.watchEvents = m.watchEvents[len(m.watchEvents)-60:]
	}

	if m.screen != screenWatch || m.watchCh == nil {
		return m, nil
	}

	return m, waitFor(m.watchCh)
}

func (m *Model) onStatusKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if isBack(msg) || msg.String() == "q" {
		m.back()
		return m, nil
	}

	switch msg.String() {
	case "r":
		return m, statusCmd(m.ctx, m.opts.Fixer, m.detections)
	case "d":
		return m, m.openDoctor()
	case "w":
		return m, m.openWatch()
	case "c":
		return m, m.openClear()
	case "u":
		return m, m.undo()
	case "?":
		m.openHelp()
	}

	return m, nil
}

func (m *Model) openClear() tea.Cmd {
	store := backup.New(m.opts.Fixer.BackupDir, m.opts.Fixer.Backups)

	snapshots, err := store.List()
	if err != nil {
		m.setNotice("could not read backups: " + err.Error())
		return nil
	}
	if len(snapshots) == 0 {
		m.setNotice("no backups to clear")
		return nil
	}

	var bytes int64
	for _, snapshot := range snapshots {
		bytes += snapshot.Bytes
	}

	m.clearCount = len(snapshots)
	m.clearBytes = bytes
	m.clearDir = store.Dir
	m.clearStage = 0
	m.clearBusy = false
	m.show(screenClear)

	return nil
}

func (m *Model) onClearKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.clearBusy {
		return m, nil
	}

	if isBack(msg) || msg.String() == "n" || msg.String() == "q" {
		m.back()
		return m, nil
	}

	if msg.String() != "y" {
		return m, nil
	}

	if m.clearStage == 0 {
		m.clearStage = 1
		return m, nil
	}

	m.clearBusy = true
	store := backup.New(m.opts.Fixer.BackupDir, m.opts.Fixer.Backups)

	return m, func() tea.Msg {
		count, bytes, err := store.Clear()
		return clearMsg{count: count, bytes: bytes, err: err}
	}
}

func (m *Model) onClear(msg clearMsg) (tea.Model, tea.Cmd) {
	m.clearBusy = false

	if msg.err != nil {
		m.setNotice("could not clear backups: " + msg.err.Error())
	} else {
		m.setNotice(fmt.Sprintf("removed %d backups (%s)", msg.count, units.Bytes(msg.bytes)))
	}

	if !m.back() {
		m.screen = screenStatus
	}

	return m, statusCmd(m.ctx, m.opts.Fixer, m.detections)
}

func (m *Model) onDoctorKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if isBack(msg) || msg.String() == "q" {
		m.back()
		return m, nil
	}

	switch msg.String() {
	case "r":
		m.checks = nil
		return m, m.doctorCmd()
	case "up", "k":
		m.scroll = max(m.scroll-1, 0)
	case "down", "j":
		m.scroll = min(m.scroll+1, m.maxScroll())
	case "pgup":
		m.scroll = max(m.scroll-m.pageRows(), 0)
	case "pgdown":
		m.scroll = min(m.scroll+m.pageRows(), m.maxScroll())
	case "s":
		return m, m.openStatus()
	case "w":
		return m, m.openWatch()
	case "?":
		m.openHelp()
	}

	return m, nil
}

func (m *Model) onWatchKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if isBack(msg) || msg.String() == "q" {
		m.stopWatch()
		m.back()
		return m, nil
	}

	switch msg.String() {
	case "up", "k":
		m.watchScroll = max(m.watchScroll-1, 0)
	case "down", "j":
		m.watchScroll = min(m.watchScroll+1, m.maxWatchScroll())
	case "pgup":
		m.watchScroll = max(m.watchScroll-m.pageRows(), 0)
	case "pgdown":
		m.watchScroll = min(m.watchScroll+m.pageRows(), m.maxWatchScroll())
	case " ", "space":
		if m.watchPaused {
			m.watchPaused = false
			return m, m.startWatch()
		}

		m.stopWatch()
		m.watchPaused = true
		return m, nil
	case "+", "=":
		m.watchInterval = clampDuration(m.watchInterval+15*time.Second, watch.MinInterval, 10*time.Minute)
		m.watchPaused = false
		return m, m.startWatch()
	case "-", "_":
		m.watchInterval = clampDuration(m.watchInterval-15*time.Second, watch.MinInterval, 10*time.Minute)
		m.watchPaused = false
		return m, m.startWatch()
	case "r":
		m.watchEvents = nil
		m.watchScroll = 0
		m.watchPaused = false
		return m, m.startWatch()
	case "s":
		m.stopWatch()
		return m, m.openStatus()
	case "d":
		m.stopWatch()
		return m, m.openDoctor()
	case "?":
		m.stopWatch()
		m.watchPaused = true
		m.openHelp()
	}

	return m, nil
}

func (m *Model) undo() tea.Cmd {
	if m.dryRun {
		m.setNotice("dry run, nothing to undo")
		return nil
	}

	options := m.opts.Fixer
	store := backup.New(options.BackupDir, options.Backups)

	id := m.result.Backup
	if id == "" {
		if snapshots, err := store.List(); err == nil && len(snapshots) > 0 {
			id = snapshots[0].ID
		}
	}

	if id == "" {
		m.setNotice("there is no backup to restore")
		return nil
	}

	logger := m.logger

	return func() tea.Msg {
		snapshot, err := store.Get(id)
		if err != nil {
			return undoMsg{err: err}
		}

		result, err := fixer.Rollback(snapshot, fixer.Plan{
			BackupDir: options.BackupDir,
			Backups:   options.Backups,
			StateDir:  options.StateDir,
			Tool:      options.Tool,
		}, logger, nil)

		return undoMsg{id: snapshot.ID, result: result, err: err}
	}
}

func (m *Model) onUndo(msg undoMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setNotice("could not restore the backup: " + msg.err.Error())
		return m, nil
	}

	m.result = msg.result
	m.undone = true
	m.verify = verifyNone
	m.setNotice("restored backup " + msg.id)

	return m, statusCmd(m.ctx, m.opts.Fixer, m.detections)
}

func (m *Model) openStream() tea.Cmd {
	install := m.plan.Helium
	stream := verify.Default()
	m.stream = stream

	if install.Exe == "" {
		if found, err := helium.Find(m.opts.Fixer.HeliumPath); err == nil {
			install = found
		}
	}

	return func() tea.Msg {
		return verifyMsg{stream: stream, err: verify.Open(install, stream)}
	}
}

func (m *Model) streamQuestion() string {
	if m.stream.Prompt == "" {
		return "did the video play? y / n"
	}

	return m.stream.Prompt
}

func (m *Model) onVerify(msg verifyMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setNotice("could not open " + msg.stream.Name + ": " + msg.err.Error())
		return m, nil
	}

	m.verify = verifyAsking
	m.setNotice("opened " + msg.stream.Name + ", " + msg.stream.Prompt)

	return m, nil
}

func (m *Model) confirmPlayback(played bool) (tea.Model, tea.Cmd) {
	if m.verify != verifyAsking {
		return m, nil
	}

	target := m.result.Target
	if target == "" {
		target = m.plan.Target
	}

	if !played {
		m.verify = verifyDeclined
		m.setNotice("not confirmed, the CDM can still work on other sites")
		return m, nil
	}

	m.verify = verifyConfirmed
	m.setNotice("playback confirmed and recorded")

	store := state.Open(m.opts.Fixer.StateDir)

	return m, func() tea.Msg {
		store.MarkVerified(target)
		return nil
	}
}

func (m *Model) setNotice(text string) {
	m.notice, m.noticeAt = text, time.Now()
}

func clampDuration(value, lo, hi time.Duration) time.Duration {
	if value < lo {
		return lo
	}
	if value > hi {
		return hi
	}

	return value
}
