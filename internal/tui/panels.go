package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"

	"hdf/internal/doctor"
	"hdf/internal/status"
	"hdf/internal/units"
	"hdf/internal/watch"
)

func (m *Model) renderStatus(l layout) string {
	width := l.text(l.content)

	budget := l.body
	if l.framed {
		budget -= frameHeight
	}

	lines := m.statusLines(width, max(budget, 1))
	if notice := m.noticeLine(width); notice != "" {
		lines = append([]string{notice}, lines...)
	}

	return panel{
		title:   "status",
		hint:    "esc to go back",
		lines:   lines,
		width:   l.content,
		height:  cardHeight(l.body, lines),
		framed:  l.framed,
		focused: true,
		phase:   m.phase(),
	}.render()
}

func (m *Model) statusLines(width, budget int) []string {
	if m.reportAt.IsZero() {
		return []string{"", " " + m.spinner.View() + " " + theme.MutedS.Render("reading the Helium install")}
	}

	report := m.report

	if !report.Found {
		lines := []string{"", " " + theme.Warn.Render("Helium was not found"), ""}

		if report.Running.Running {
			lines = append(lines, " "+theme.Good.Render("●")+" "+
				theme.MutedS.Render("but a copy is running from "+report.Running.Describe()))
			lines = append(lines, " "+theme.MutedS.Render("pass --helium with that directory"))
			return lines
		}

		return append(lines, " "+theme.MutedS.Render("pass --helium with the install directory"))
	}

	head := []string{"", " " + theme.Heading.Render(clip(report.Helium.Root, width-1)), ""}

	if report.CDM == nil {
		lines := append(head, labeled("installed", "nothing yet", width)...)
		lines = append(lines, "", " "+theme.MutedS.Render("pick a source to install one"))

		return lines
	}

	installed := report.CDM.Version
	if installed == "" {
		installed = "unknown version"
	}

	origin := "by something else"
	if report.CDM.Managed {
		origin = "by hdf"
		if report.CDM.Source != "" {
			origin = "from " + report.CDM.Source
		}
	}

	rows := [][2]string{
		{"installed", installed},
		{"origin", origin},
		{"when", units.Since(report.CDM.At)},
		{"size", fmt.Sprintf("%s in %d files", units.Bytes(report.CDM.Size), report.CDM.Files)},
		{"target", report.CDM.Path},
		{"backups", fmt.Sprintf("%d saved", report.Backups)},
		{"running", runningFact(report)},
	}

	if report.Newest.Available {
		rows = append(rows, [2]string{"newest", fmt.Sprintf("%s (%s)", report.Newest.Version, report.Newest.Name)})
	}

	rows = trimRows(rows, head, width, budget)

	lines := head
	lines = append(lines, facts(rows, width)...)

	lines = append(lines, "")

	switch {
	case report.Outdated():
		lines = append(lines, " "+theme.Warn.Render("◆")+" "+theme.Warn.Render("update available")+
			" "+theme.MutedS.Render(fmt.Sprintf("%s from %s", report.Newest.Version, report.Newest.Name)))
	case report.Newest.Available:
		lines = append(lines, " "+theme.Good.Render("✓")+" "+theme.MutedS.Render("up to date with the newest browser copy"))
	default:
		lines = append(lines, " "+theme.FaintS.Render("·")+" "+theme.MutedS.Render("no browser can supply a CDM right now"))
	}

	if report.CDM.Verified {
		lines = append(lines, " "+theme.Good.Render("✓")+" "+theme.MutedS.Render("playback confirmed in a test stream"))
	}

	return lines
}

func runningFact(report status.Report) string {
	if !report.Running.Running {
		return "no"
	}

	pids := ""
	if n := len(report.Running.PIDs); n == 1 {
		pids = fmt.Sprintf(" (pid %d)", report.Running.PIDs[0])
	} else if n > 1 {
		pids = fmt.Sprintf(" (%d processes)", n)
	}

	return "yes" + pids + ", restart to load a new CDM"
}

func facts(rows [][2]string, width int) []string {
	if len(rows) == 0 {
		return nil
	}

	data := make([][]string, 0, len(rows))
	for _, row := range rows {
		data = append(data, []string{theme.Label.Render(row[0]), theme.Value.Render(clip(row[1], max(width-14, 12)))})
	}

	rendered := table.New().
		Rows(data...).
		Border(lipgloss.HiddenBorder()).
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false).
		BorderColumn(false).
		BorderRow(false).
		BorderHeader(false).
		StyleFunc(func(_, col int) lipgloss.Style {
			if col == 0 {
				return lipgloss.NewStyle().Width(11).PaddingRight(1).Faint(true)
			}
			return lipgloss.NewStyle()
		}).
		Render()

	lines := strings.Split(strings.TrimRight(rendered, "\n"), "\n")
	for i := range lines {
		lines[i] = " " + clip(lines[i], width-1)
	}

	return lines
}

func trimRows(rows [][2]string, head []string, width, budget int) [][2]string {
	if budget <= 0 {
		return rows
	}

	for _, label := range []string{"size", "backups", "when", "newest", "origin", "target"} {
		if countLines(head, rows, width)+3 <= budget {
			break
		}

		for i, row := range rows {
			if row[0] != label {
				continue
			}
			rows = append(rows[:i], rows[i+1:]...)
			break
		}
	}

	return rows
}

func countLines(head []string, rows [][2]string, width int) int {
	total := len(head)

	for _, row := range rows {
		total += len(labeled(row[0], row[1], width))
	}

	return total
}

func (m *Model) renderDoctor(l layout) string {
	width := l.text(l.content)

	if len(m.checks) == 0 {
		lines := []string{"", " " + m.spinner.View() + " " + theme.MutedS.Render("running checks")}

		return panel{
			title:   "doctor",
			hint:    "checking",
			lines:   lines,
			width:   l.content,
			height:  cardHeight(l.body, lines),
			framed:  l.framed,
			focused: true,
			phase:   m.phase(),
		}.render()
	}

	lines := m.doctorLines(width)

	rows := m.pageRows()
	scroll := clamp(m.scroll, 0, max(len(lines)-rows, 0))

	window := lines[scroll:]
	if len(window) > rows {
		window = window[:rows]
	}

	hint := "esc to go back"
	if len(lines) > rows {
		hint = fmt.Sprintf("%d-%d of %d", scroll+1, scroll+len(window), len(lines))
	}

	height, align := cardHeight(l.body, window), vCenter
	if len(lines) > rows {
		height, align = l.body, 0
	}

	return panel{
		title:   "doctor",
		hint:    hint,
		lines:   window,
		width:   l.content,
		height:  height,
		framed:  l.framed,
		focused: true,
		valign:  align,
		phase:   m.phase(),
	}.render()
}

func (m *Model) doctorLines(width int) []string {
	fails, warns := doctor.Summary(m.checks)

	lines := []string{"", summaryLine(len(m.checks), fails, warns), " " + rule(width-1), ""}

	data := make([][]string, 0, len(m.checks))
	for _, check := range m.checks {
		data = append(data, []string{
			iconFor(check.Level),
			theme.Label.Render(padRight(check.Name, 14)),
			detailFor(check),
		})
	}

	rendered := table.New().
		Rows(data...).
		Border(lipgloss.HiddenBorder()).
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false).
		BorderColumn(false).
		BorderRow(false).
		BorderHeader(false).
		StyleFunc(func(_, col int) lipgloss.Style {
			switch col {
			case 0:
				return lipgloss.NewStyle().PaddingRight(1)
			case 1:
				return lipgloss.NewStyle().PaddingRight(1)
			}
			return lipgloss.NewStyle()
		}).
		Width(max(width-2, 20)).
		Render()

	for _, line := range strings.Split(strings.TrimRight(rendered, "\n"), "\n") {
		lines = append(lines, " "+clip(line, width-1))
	}

	for _, check := range m.checks {
		if check.Hint == "" {
			continue
		}
		for _, hint := range wrapText(check.Hint, max(width-8, 10)) {
			lines = append(lines, "     "+theme.FaintS.Render(hint))
		}
	}

	return lines
}

func iconFor(level doctor.Level) string {
	switch level {
	case doctor.LevelOK:
		return theme.Good.Render("✓")
	case doctor.LevelWarn:
		return theme.Warn.Render("!")
	case doctor.LevelFail:
		return theme.Bad.Render("✗")
	}

	return theme.FaintS.Render("·")
}

func detailFor(check doctor.Check) string {
	switch check.Level {
	case doctor.LevelFail:
		return theme.Bad.Render(check.Detail)
	case doctor.LevelWarn:
		return theme.Value.Render(check.Detail)
	}

	return theme.MutedS.Render(check.Detail)
}

func summaryLine(checks, fails, warns int) string {
	verdict := theme.Good.Render("none failed")
	if fails > 0 {
		verdict = theme.Bad.Render(fmt.Sprintf("%d failed", fails))
	}

	parts := []string{theme.MutedS.Render(fmt.Sprintf("%d checks", checks)), verdict}
	if warns > 0 {
		parts = append(parts, theme.Warn.Render(fmt.Sprintf("%d to look at", warns)))
	}

	return " " + strings.Join(parts, theme.MutedS.Render("  ·  "))
}

func (m *Model) renderWatch(l layout) string {
	width := l.text(l.content)
	lines := m.watchLines(width)
	rows := m.pageRows()

	scroll := clamp(m.watchScroll, 0, max(len(lines)-rows, 0))
	m.watchScroll = scroll

	window := lines[scroll:]
	if len(window) > rows {
		window = window[:rows]
	}

	hint := "esc to stop"
	if len(lines) > rows {
		hint = fmt.Sprintf("%d-%d of %d", scroll+1, scroll+len(window), len(lines))
	}

	height := cardHeight(l.body, window)
	if len(lines) > rows {
		height = l.body
	}

	return panel{
		title:   "watch",
		hint:    hint,
		lines:   window,
		width:   l.content,
		height:  height,
		framed:  l.framed,
		focused: true,
		phase:   m.phase(),
	}.render()
}

func (m *Model) renderClear(l layout) string {
	width := l.text(l.content)
	lines := []string{"", " " + theme.Heading.Render("Clear saved backups"), ""}

	if m.clearCount == 0 {
		lines = append(lines, " "+theme.MutedS.Render("nothing to clear"))
	} else {
		lines = append(lines, labeled("backups", fmt.Sprintf("%d (%s)", m.clearCount, units.Bytes(m.clearBytes)), width)...)
		lines = append(lines, labeled("folder", m.clearDir, width)...)
		lines = append(lines, "")

		switch {
		case m.clearBusy:
			lines = append(lines, " "+m.spinner.View()+" "+theme.MutedS.Render("removing backups"))
		case m.clearStage > 0:
			lines = append(lines,
				" "+theme.Warn.Render("This cannot be undone."),
				"",
				" "+theme.Heading.Render("Press y again to delete every backup."),
			)
		default:
			lines = append(lines,
				" "+theme.Warn.Render("This cannot be undone."),
				"",
				" "+theme.MutedS.Render("Press y to continue."),
			)
		}
	}

	hint := "esc to cancel"
	if m.clearBusy {
		hint = "clearing"
	} else if m.clearStage > 0 {
		hint = "confirm"
	}

	return panel{
		title:   "clear backups",
		hint:    hint,
		lines:   lines,
		width:   l.content,
		height:  cardHeight(l.body, lines),
		framed:  l.framed,
		focused: true,
		phase:   m.phase(),
	}.render()
}

func (m *Model) watchLines(width int) []string {
	armed := theme.Good.Render("● armed")
	if m.watchPaused {
		armed = theme.Warn.Render("⏸ paused")
	}

	lines := []string{
		"",
		" " + armed + theme.MutedS.Render("  every "+units.Duration(m.watchInterval)),
		" " + rule(width-1),
		"",
	}

	if len(m.watchEvents) == 0 {
		lines = append(lines, " "+theme.MutedS.Render("waiting for the first check"))
		return lines
	}

	for i := len(m.watchEvents) - 1; i >= 0; i-- {
		lines = append(lines, watchRow(m.watchEvents[i], width))
	}

	return lines
}

func watchRow(event watch.Event, width int) string {
	icon, style := theme.FaintS.Render("·"), theme.MutedS
	switch event.Kind {
	case watch.KindApplied:
		icon, style = theme.Good.Render("✓"), theme.Value
	case watch.KindError:
		icon, style = theme.Bad.Render("✗"), theme.Bad
	case watch.KindArmed:
		icon, style = theme.Accent.Render("▌"), theme.MutedS
	}

	stamp := event.At.Format("15:04:05")
	message := clip(event.Message, max(width-18, 8))

	return " " + theme.FaintS.Render(stamp) + "  " + icon + " " + style.Render(message)
}

func (m *Model) maxScroll() int {
	lines := len(m.doctorLines(m.layout().text(m.layout().content)))

	return max(lines-m.pageRows(), 0)
}

func (m *Model) maxWatchScroll() int {
	lines := len(m.watchLines(m.layout().text(m.layout().content)))

	return max(lines-m.pageRows(), 0)
}

func (m *Model) pageRows() int {
	l := m.layout()
	if l.framed {
		return max(l.body-frameHeight, 1)
	}

	return max(l.body, 1)
}

func (m *Model) noticeLine(width int) string {
	if m.notice == "" {
		return ""
	}

	return " " + theme.Accent.Render("→") + " " + theme.MutedS.Render(clip(m.notice, max(width-4, 8)))
}
