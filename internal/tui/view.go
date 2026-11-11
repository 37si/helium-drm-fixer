package tui

import (
	"fmt"
	"math"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func (m *Model) View() tea.View {
	view := tea.NewView(m.render())
	view.AltScreen = true
	view.WindowTitle = m.title()
	view.ReportFocus = true

	view.MouseMode = tea.MouseModeAllMotion
	if m.opts.NoMouse {
		view.MouseMode = tea.MouseModeNone
	}

	view.ProgressBar = m.progressBar()
	view.ProgramStatus = m.programStatus()

	return view
}

func (m *Model) title() string {
	switch m.screen {
	case screenScanning:
		return "hdf · looking for WidevineCdm"
	case screenPicking:
		return "hdf · choose a source"
	case screenReview:
		return "hdf · ready to install"
	case screenApplying:
		return fmt.Sprintf("hdf · installing %d%%", m.percent())
	case screenDone:
		return "hdf · installed"
	case screenFailed:
		return "hdf · failed"
	case screenStatus:
		return "hdf · status"
	case screenDoctor:
		return "hdf · doctor"
	case screenWatch:
		return "hdf · watching"
	case screenClear:
		return "hdf · clear backups"
	case screenHelp:
		return "hdf · help"
	}

	return "hdf"
}

func (m *Model) percent() int {
	return int(math.Round(min(max(m.spring.pos, 0), 1) * 100))
}

func (m *Model) progressBar() *tea.ProgressBar {
	switch m.screen {
	case screenScanning:
		return tea.NewProgressBar(tea.ProgressBarIndeterminate, 0)
	case screenApplying:
		return tea.NewProgressBar(tea.ProgressBarDefault, m.percent())
	case screenFailed:
		return tea.NewProgressBar(tea.ProgressBarError, 100)
	case screenDone:
		return tea.NewProgressBar(tea.ProgressBarNone, 100)
	}

	return nil
}

func (m *Model) programStatus() *tea.ProgramStatus {
	status := &tea.ProgramStatus{App: "hdf"}

	switch m.screen {
	case screenScanning, screenApplying:
		status.State = tea.ProgramStateWorking
		status.Title = "hdf"
		status.Message = "looking for WidevineCdm"
		if m.screen == screenApplying {
			status.Message = fmt.Sprintf("installing WidevineCdm %d%%", m.percent())
			status.Progress = m.percent()
			status.HasProgress = true
		}
	case screenReview:
		status.State = tea.ProgramStateBlocked
		status.Kind = tea.ProgramStatusKindPermission
		status.Title = "hdf"
		status.Message = "ready to install WidevineCdm"
	case screenDone:
		status.State = tea.ProgramStateDone
		status.Title = "hdf"
		status.Message = "WidevineCdm installed"
	case screenFailed:
		status.State = tea.ProgramStateError
		status.Title = "hdf"
		status.Message = "install failed"
	default:
		status.State = tea.ProgramStateIdle
		status.Title = "hdf"
		status.Message = "waiting"
	}

	return status
}

func (m *Model) render() string {
	l := m.layout()

	m.hits = m.hits[:0]

	blocks := make([]string, 0, 3)
	top := 0

	if l.header {
		blocks = append(blocks, m.renderHeader(l))
		top += 3
	}

	body, above := m.pinBodyAt(m.renderBody(l), l)
	blocks = append(blocks, body)

	m.shiftHits(top + above)

	if l.footer {
		blocks = append(blocks, m.renderFooter(l))
	}

	return fitFrame(strings.Join(blocks, "\n\n"), l.width, l.height)
}

func (m *Model) shiftHits(dy int) {
	if dy == 0 {
		return
	}

	for i := range m.hits {
		m.hits[i].y += dy
	}
}

func (m *Model) pinBodyAt(body string, l layout) (string, int) {
	rows := strings.Count(body, "\n") + 1
	if rows >= l.body {
		return body, 0
	}

	spare := l.body - rows
	above := spare / 2

	return lipgloss.PlaceVertical(l.body, lipgloss.Center, body), above
}

func fitFrame(frame string, width, height int) string {
	lines := strings.Split(frame, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}

	for i, line := range lines {
		lines[i] = padRight(ansi.Truncate(line, width, ""), width)
	}

	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}

	return strings.Join(lines, "\n")
}

func (m *Model) renderHeader(l layout) string {
	left := theme.Accent.Render("▌") + " " + wordmarkAt("hdf", m.phase())

	if l.content >= 48 {
		left += "  " + theme.MutedS.Render("WidevineCdm for Helium")
	}

	right := ""
	if m.opts.Version != "" {
		right = theme.FaintS.Render("v" + m.opts.Version)
	}
	if pill := m.runningPill(); pill != "" {
		if right != "" {
			right += "   "
		}
		right += pill
	}

	if !m.focused {
		right = theme.FaintS.Render("paused") + "   " + right
	}

	line := left
	if gap := l.content - lipgloss.Width(left) - lipgloss.Width(right); gap > 0 {
		line += strings.Repeat(" ", gap) + right
	}

	return indent(line, l.offset) + "\n" + indent(hairlineAt(l.content, m.phase()), l.offset)
}

func (m *Model) runningPill() string {
	if !m.running.Running {
		return ""
	}

	label := "running"
	if m.running.Matches {
		label = "helium running"
	}

	return theme.Good.Render("●") + " " + theme.MutedS.Render(label)
}

func (m *Model) renderBody(l layout) string {
	var body string

	switch m.screen {
	case screenScanning:
		body = m.renderScanning(l)
	case screenPicking:
		body = m.renderPicking(l)
	case screenReview:
		body = m.renderReview(l)
	case screenApplying:
		body = m.renderApplying(l)
	case screenDone:
		body = m.renderDone(l)
	case screenFailed:
		body = m.renderFailed(l)
	case screenStatus:
		body = m.renderStatus(l)
	case screenDoctor:
		body = m.renderDoctor(l)
	case screenWatch:
		body = m.renderWatch(l)
	case screenClear:
		body = m.renderClear(l)
	case screenHelp:
		body = panel{
			title:   "help",
			hint:    "esc to close",
			lines:   strings.Split(m.viewport.View(), "\n"),
			width:   l.content,
			height:  l.body,
			framed:  l.framed,
			focused: true,
			phase:   m.phase(),
		}.render()
	}

	return indent(body, l.offset)
}

func (m *Model) renderFooter(l layout) string {
	sep := theme.MutedS.Render("  ·  ")
	sepWidth := lipgloss.Width(sep)

	parts := make([]string, 0, 8)
	used := 0
	x := l.offset

	for i, key := range m.keyHints() {
		name := "hint:" + key[1]
		cap := keycap(key[0], key[1])
		if m.hover == name {
			cap = hotKeycap(key[0], key[1])
		}

		need := lipgloss.Width(cap)
		if i > 0 {
			need += sepWidth
		}
		if used+need > l.content && len(parts) > 0 {
			break
		}

		if i > 0 {
			x += sepWidth
		}

		width := lipgloss.Width(cap)
		m.addHit(x, l.height-1, width, name, press(key[0]))
		x += width

		parts = append(parts, cap)
		used += need
	}

	line := strings.Join(parts, sep)

	if m.opts.LogPath != "" && m.screen != screenApplying {
		log := theme.MutedS.Render("log " + truncate(m.opts.LogPath, 34))
		if gap := l.content - lipgloss.Width(line) - lipgloss.Width(log); gap > 2 {
			line += strings.Repeat(" ", gap) + log
		}
	}

	return indent(rule(l.content), l.offset) + "\n" + indent(clip(line, l.content), l.offset)
}

func (m *Model) keyHints() [][2]string {
	switch m.screen {
	case screenScanning:
		return [][2]string{{"q", "quit"}}
	case screenPicking:
		return [][2]string{{"↑↓", "move"}, {"/", "filter"}, {"↵", "continue"}, {"f", "fresh download"}, {"s", "status"}, {"d", "doctor"}, {"?", "help"}, {"q", "quit"}}
	case screenReview:
		return [][2]string{{"tab", "next"}, {"↵", "apply"}, {"esc", "back"}, {"?", "help"}}
	case screenApplying:
		return [][2]string{{"", "writing WidevineCdm"}}
	case screenDone, screenFailed:
		hints := make([][2]string, 0, 8)
		if !m.dryRun {
			hints = append(hints, [2]string{"v", "test playback"})
		}
		if m.result.Backup != "" && !m.undone {
			hints = append(hints, [2]string{"u", "undo"})
		}
		hints = append(hints,
			[2]string{"s", "status"},
			[2]string{"d", "doctor"},
			[2]string{"r", "rescan"},
			[2]string{"esc", "back"},
			[2]string{"?", "help"},
			[2]string{"q", "quit"},
		)
		return hints
	case screenStatus:
		return [][2]string{{"r", "refresh"}, {"d", "doctor"}, {"w", "watch"}, {"c", "clear backups"}, {"u", "undo"}, {"esc", "back"}}
	case screenDoctor:
		return [][2]string{{"↑↓", "scroll"}, {"r", "rerun"}, {"s", "status"}, {"esc", "back"}}
	case screenClear:
		if m.clearBusy {
			return [][2]string{{"", "removing backups"}}
		}
		if m.clearStage > 0 {
			return [][2]string{{"y", "delete now"}, {"esc", "cancel"}}
		}
		return [][2]string{{"y", "continue"}, {"esc", "cancel"}}
	case screenWatch:
		hints := [][2]string{{"↑↓", "scroll"}}
		if m.watchPaused {
			hints = append(hints, [2]string{"space", "resume"})
		} else {
			hints = append(hints, [2]string{"space", "pause"})
		}
		return append(hints, [2]string{"+/-", "interval"}, [2]string{"?", "help"}, [2]string{"esc", "stop"})
	case screenHelp:
		return [][2]string{{"esc", "back"}, {"↑↓", "scroll"}}
	}

	return [][2]string{{"q", "quit"}}
}

func sideBySide(gap int, left, right string) string {
	if gap <= 0 {
		return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	}

	height := max(lipgloss.Height(left), lipgloss.Height(right))
	gutter := strings.TrimSuffix(strings.Repeat(strings.Repeat(" ", gap)+"\n", height), "\n")

	return lipgloss.JoinHorizontal(lipgloss.Top, left, gutter, right)
}
