package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	lipglosslist "charm.land/lipgloss/v2/list"
	"charm.land/lipgloss/v2/tree"

	"hdf/internal/browser"
	"hdf/internal/fixer"
	"hdf/internal/pathutil"
	"hdf/internal/proc"
	"hdf/internal/units"
)

func (m *Model) renderScanning(l layout) string {
	rows := make([]string, 0, len(browser.All))
	for _, b := range browser.All {
		rows = append(rows, m.scanRow(b, l))
	}

	capacity := l.body
	if l.framed {
		capacity -= frameHeight
	}
	if keep := max(capacity-2, 1); len(rows) > keep {
		rows = rows[:keep]
	}

	fraction := 0.0
	if m.scanTotal > 0 {
		fraction = float64(m.scanned) / float64(m.scanTotal)
	}

	lines := append(rows, "",
		" "+m.progress.ViewAs(fraction)+"  "+
			theme.MutedS.Render(fmt.Sprintf("%d of %d", m.scanned, m.scanTotal)),
	)

	return panel{
		title:   "sources",
		hint:    "scanning",
		lines:   lines,
		width:   l.content,
		height:  cardHeight(l.body, lines),
		framed:  l.framed,
		focused: true,
		phase:   m.phase(),
	}.render()
}

func (m *Model) scanRow(b browser.Browser, l layout) string {
	budget := max(l.text(l.content)-1, 8)
	detection, done := m.seen[b.ID]

	dot, nameStyle, note, noteStyle := theme.FaintS.Render("·"), theme.FaintS, "", theme.FaintS

	if at, ok := m.flash[b.ID]; ok && m.anim {
		if age := m.frame - at; age >= 0 && age < flashLife {
			fade := 1 - float64(age)/flashLife
			dot = lipgloss.NewStyle().Foreground(blend(theme.Primary, theme.Text, fade)).Render("◍")
			nameStyle = lipgloss.NewStyle().Foreground(blend(theme.Muted, theme.Text, fade))
		}
	}

	switch {
	case !done:
	case detection.CDM != nil:
		dot, nameStyle = theme.Good.Render("●"), theme.Value
		note = detection.CDM.Version
		noteStyle = theme.MutedS
	case detection.Executable != "":
		dot, nameStyle = theme.Warn.Render("○"), theme.MutedS
		note, noteStyle = "no CDM", theme.Warn
	default:
		dot, nameStyle = theme.FaintS.Render("○"), theme.FaintS
	}

	noteWidth := lipgloss.Width(note)
	nameWidth := budget - 3
	if noteWidth > 0 {
		nameWidth = budget - 4 - noteWidth
	}

	name := padRight(clip(b.Name, max(nameWidth, 8)), max(nameWidth, 8))
	row := " " + dot + " " + nameStyle.Render(name)

	if noteWidth > 0 {
		row += " " + noteStyle.Render(note)
	}

	return row
}

func (m *Model) renderPicking(l layout) string {
	usable, total := 0, len(m.detections)
	if m.hasFresh {
		usable++
		total++
	}

	for _, detection := range m.detections {
		if detection.Available() {
			usable++
		}
	}

	pane := l.text(l.left)
	lines := listLines(m.listView())
	rowAt := 0

	inner := l.body
	if l.framed {
		inner -= frameHeight
	}

	if spare := inner - len(lines) - 1; spare > 0 {
		lines = append([]string{""}, lines...)
		rowAt = 1

		lines = append(lines, m.heldBlock(pane, spare)...)

		for len(lines) < inner-1 {
			lines = append(lines, "")
		}

		lines = append(lines, legend(pane))
	}

	m.addRowHits(l, rowAt, len(lines)-rowAt)

	left := panel{
		title:   "sources",
		hint:    fmt.Sprintf("%d of %d usable", usable, total),
		lines:   lines,
		width:   l.left,
		height:  l.body,
		framed:  l.framed,
		focused: true,
		phase:   m.phase(),
	}.render()

	if !l.detail {
		return left
	}

	right := panel{
		title:  "details",
		hint:   "enter to continue",
		lines:  m.detailPane(l.text(l.right), l.body),
		width:  l.right,
		height: l.body,
		framed: l.framed,
	}.render()

	return sideBySide(l.gap, left, right)
}

func (m *Model) addRowHits(l layout, rowAt, visible int) {
	if l.stacked || m.list.FilterState() != list.Unfiltered {
		return
	}

	start := m.list.Paginator.Page * m.list.Paginator.PerPage
	total := len(m.list.Items())
	top := 0
	if l.framed {
		top = 1
	}

	for j := 0; j < visible; j++ {
		index := start + j
		if index >= total {
			return
		}

		m.addHit(l.offset, top+rowAt+j, l.left, rowName(index), clickRow(index))
	}
}

func clickRow(index int) func(*Model) tea.Cmd {
	return func(m *Model) tea.Cmd {
		if m.list.Index() != index {
			m.list.Select(index)
			return nil
		}

		msg, ok := keyPress("↵")
		if !ok {
			return nil
		}

		_, cmd := m.onKey(msg)

		return cmd
	}
}

func (m *Model) heldBlock(width, room int) []string {
	if m.heliumDir == "" || room < 5 {
		return nil
	}

	value := "nothing — first install"
	if m.held != "" {
		value = m.held

		if picked := m.pickedVersion(); picked != "" && pathutil.CompareVersions(picked, m.held) > 0 {
			value += "   " + theme.FaintS.Render("·") + " " + theme.MutedS.Render("newer in the list")
		}
	}

	block := []string{""}
	block = append(block, labeled("helium", m.heliumDir, width)...)
	block = append(block, labeled("has", value, width)...)
	block = append(block, "")

	if len(block) > room {
		return nil
	}

	return block
}

func (m *Model) pickedVersion() string {
	item, ok := m.list.SelectedItem().(sourceItem)
	if !ok || item.detection.CDM == nil {
		return ""
	}

	return item.detection.CDM.Version
}

func (m *Model) detailPane(width, height int) []string {
	if item, ok := m.list.SelectedItem().(sourceItem); ok && item.fresh {
		return m.freshDetail(width, height)
	}

	block := []string{""}
	block = append(block, m.detailHead(width)...)
	block = append(block, "")

	if facts := m.detailFacts(width); len(facts) > 0 {
		block = append(block, facts...)
	}

	return pinFooter(block, m.detailFooter(width), max(height-frameHeight, 0))
}

func (m *Model) detailFooter(width int) []string {
	item, ok := m.list.SelectedItem().(sourceItem)
	if !ok || !item.detection.Available() {
		return nil
	}

	return []string{
		" " + rule(width-1),
		" " + keycap("↵", "installs this copy"),
	}
}

func legend(width int) string {
	if width < 34 {
		return " " + theme.Good.Render("●") + theme.MutedS.Render(" ready ") +
			theme.FaintS.Render("○") + theme.MutedS.Render(" none")
	}

	return " " + theme.Good.Render("●") + theme.MutedS.Render(" ready") +
		"    " + theme.FaintS.Render("○") + theme.MutedS.Render(" no WidevineCdm")
}

func listLines(view string) []string {
	lines := strings.Split(view, "\n")

	for len(lines) > 0 && blank(lines[0]) {
		lines = lines[1:]
	}
	for len(lines) > 0 && blank(lines[len(lines)-1]) {
		lines = lines[:len(lines)-1]
	}

	return lines
}

func (m *Model) detailHead(width int) []string {
	item, ok := m.list.SelectedItem().(sourceItem)
	if !ok {
		return []string{"", theme.MutedS.Render(" no browser selected")}
	}

	detection := item.detection

	status := theme.Good.Render("ready")
	if !detection.Available() {
		status = theme.Warn.Render("no CDM")
	}

	gap := width - 1 - lipgloss.Width(detection.Name) - lipgloss.Width(status)
	head := " " + theme.Heading.Render(clip(detection.Name, max(width-8, 8)))
	if gap > 0 {
		head += strings.Repeat(" ", gap) + status
	}

	return []string{head, " " + hairline(max(width-1, 1))}
}

func (m *Model) detailFacts(width int) []string {
	item, ok := m.list.SelectedItem().(sourceItem)
	if !ok {
		return nil
	}

	detection := item.detection

	rows := make([][2]string, 0, 4)
	if detection.CDM != nil {
		version := detection.CDM.Version
		if version == "" {
			version = "unknown"
		}
		rows = append(rows, [2]string{"version", version})
		rows = append(rows, [2]string{"origin", string(detection.CDM.Origin)})
	}
	if detection.ID == m.last.id && !m.last.at.IsZero() {
		rows = append(rows, [2]string{"last", "used " + units.Since(m.last.at)})
	}
	if detection.Executable != "" {
		rows = append(rows, [2]string{"binary", detection.Executable})
	}
	if detection.CDM != nil {
		rows = append(rows, [2]string{"path", detection.CDM.Path})
	}

	var lines []string
	for _, row := range rows {
		lines = append(lines, labeled(row[0], row[1], width)...)
	}

	if detection.CDM != nil {
		lines = append(lines, "", " "+theme.FaintS.Render(clip(originNote(detection.CDM.Origin), width-1)))
	}

	if !detection.Available() {
		lines = append(lines, "",
			" "+theme.Warn.Render("no WidevineCdm here"),
			" "+theme.MutedS.Render("pick a browser marked ● instead"),
		)
	}

	return lines
}

func (m *Model) renderReview(l layout) string {
	planWidth, optionWidth := l.left, l.right
	if l.detail {
		planWidth = l.content * 60 / 100
		optionWidth = l.content - planWidth - l.gap
	}

	planLines := m.planLines(l.text(planWidth))
	formLines := m.formLines()

	height := cardHeight(l.body, planLines)
	if need := len(formLines) + frameHeight; need > height && need <= l.body {
		height = need
	}
	height = clamp(height, 6, l.body)

	planPanel := panel{
		title:   "plan",
		hint:    m.planHint(),
		lines:   planLines,
		width:   planWidth,
		height:  height,
		framed:  l.framed,
		focused: true,
		phase:   m.phase(),
	}

	optionsPanel := panel{
		title:  "options",
		hint:   "tab to move",
		lines:  formLines,
		width:  optionWidth,
		height: height,
		framed: l.framed,
	}

	if l.detail {
		return sideBySide(l.gap, planPanel.render(), optionsPanel.render())
	}

	half := l.body / 2
	if half < 3 {
		optionsPanel.width, optionsPanel.height = l.content, l.body
		return optionsPanel.render()
	}

	planPanel.width = l.content
	planPanel.height = clamp(fitHeight(half, planLines), 3, half)

	optionsPanel.width, optionsPanel.height = l.content, l.body-planPanel.height

	return lipgloss.JoinVertical(lipgloss.Left, planPanel.render(), optionsPanel.render())
}

func (m *Model) planHint() string {
	switch {
	case !m.planReady:
		return "resolving"
	case m.dryRun:
		return "dry run, nothing is written"
	}

	return "ready to write"
}

func (m *Model) planLines(width int) []string {
	if !m.planReady {
		return []string{"", " " + m.spinner.View() + " " + theme.MutedS.Render("resolving source and target")}
	}

	plan := m.plan

	root := tree.Root(theme.Heading.Render("install")).
		EnumeratorStyle(theme.FaintS).
		ItemStyle(theme.Value).
		Enumerator(tree.RoundedEnumerator)

	root.Child(tree.Root(theme.Label.Render("source") + "  " + sourceLabel(plan.Source)))
	root.Child(tree.Root(theme.Label.Render("target") + "  " + plan.Target))

	if plan.HintFile != "" {
		root.Child(tree.Root(theme.Label.Render("hint") + "  " + plan.HintFile))
	}
	if plan.NeedsResign {
		root.Child(tree.Root(theme.Label.Render("resign") + "  " + theme.Warn.Render("Helium.app will be re-signed")))
	}
	if plan.Helium.Root != "" {
		root.Child(tree.Root(theme.Label.Render("helium") + "  " + plan.Helium.Root))
	}

	if m.opts.Fixer.Backup {
		root.Child(tree.Root(theme.Label.Render("backup") + "  " + fmt.Sprintf("yes, %d kept", m.opts.Fixer.Backups)))
	} else {
		root.Child(tree.Root(theme.Label.Render("backup") + "  " + theme.MutedS.Render("off")))
	}

	lines := []string{""}
	for _, line := range strings.Split(root.String(), "\n") {
		lines = append(lines, " "+clip(line, max(width-2, 8)))
	}

	lines = append(lines, "")

	if m.dryRun {
		lines = append(lines, " "+theme.Warn.Render("◆ dry run, nothing will be written"))
	} else {
		lines = append(lines, " "+theme.Good.Render("◆ ready to write"))
	}

	if m.running.Running {
		lines = append(lines, "", " "+theme.Warn.Render("▲ Helium is running, restart it afterwards"))
	}

	return lines
}

func (m *Model) formLines() []string {
	if m.form == nil {
		return []string{"", " " + theme.MutedS.Render("waiting for the plan")}
	}

	return strings.Split(m.form.View(), "\n")
}

func (m *Model) renderApplying(l layout) string {
	lines := []string{""}

	for _, step := range m.steps {
		switch step.state {
		case stepDone:
			lines = append(lines, " "+theme.Good.Render("✓")+" "+theme.MutedS.Render(step.name))
		case stepActive:
			lines = append(lines, " "+m.spinner.View()+" "+theme.Accent.Bold(true).Render(step.name))
		default:
			lines = append(lines, " "+theme.FaintS.Render("○ "+step.name))
		}
	}

	percent := int(m.spring.pos*100 + 0.5)

	lines = append(lines, "",
		" "+m.progress.ViewAs(m.spring.pos)+"  "+
			theme.Value.Render(fmt.Sprintf("%d%%", percent))+"  "+
			theme.MutedS.Render(m.elapsed()),
		"",
	)

	width := l.text(l.content)
	lines = append(lines, labeled("from", sourceLabel(m.plan.Source), width)...)
	lines = append(lines, labeled("to", m.plan.Target, width)...)

	return panel{
		title:   "installing",
		hint:    "writing WidevineCdm",
		lines:   lines,
		width:   l.content,
		height:  cardHeight(l.body, lines),
		framed:  l.framed,
		focused: true,
		phase:   m.phase(),
	}.render()
}

func (m *Model) renderDone(l layout) string {
	headline := theme.Good.Bold(true).Render("✓") + "  " + wordmarkAt("WidevineCdm installed", m.phase())

	var rows [][2]string
	switch {
	case m.undone:
		headline = theme.Good.Bold(true).Render("✓") + "  " + theme.Heading.Render("backup restored")
		rows = [][2]string{
			{"backup", m.result.Backup},
			{"version", m.result.Version},
			{"target", m.result.Target},
		}
	case m.dryRun:
		headline = theme.Warn.Bold(true).Render("◆") + "  " + theme.Heading.Render("dry run, nothing was written")
		rows = [][2]string{
			{"source", sourceLabel(m.plan.Source)},
			{"target", m.plan.Target},
		}
	default:
		rows = [][2]string{
			{"source", sourceLabel(m.plan.Source)},
			{"target", m.plan.Target},
		}
	}

	if !m.undone {
		if m.plan.HintFile != "" {
			rows = append(rows, [2]string{"hint", m.plan.HintFile})
		}
		if m.plan.Helium.Root != "" {
			rows = append(rows, [2]string{"helium", m.plan.Helium.Root})
		}
		if m.result.Backup != "" {
			rows = append(rows, [2]string{"backup", m.result.Backup + ", press u to undo"})
		}

		rows = append(rows, [2]string{"took", m.elapsed()})
	}

	width := l.text(l.content)
	lines := []string{"", " " + headline, ""}

	for _, row := range rows {
		lines = append(lines, labeled(row[0], row[1], width)...)
	}

	for _, warning := range m.result.Warnings {
		lines = append(lines, "", " "+theme.Warn.Render("! "+clip(warning, width-2)))
	}

	lines = append(lines, "")

	if notice := m.noticeLine(width); notice != "" {
		lines = append(lines, notice, "")
	}

	switch m.verify {
	case verifyAsking:
		lines = append(lines, " "+theme.Accent.Render("?")+" "+
			theme.Heading.Render(m.streamQuestion()), "")
	case verifyConfirmed:
		lines = append(lines, " "+theme.Good.Render("✓")+" "+
			theme.MutedS.Render("playback confirmed"), "")
	}

	lines = append(lines, " "+restartAdvice(m.running))

	if m.verify != verifyConfirmed && !m.undone && !m.dryRun {
		lines = append(lines, "")
		for _, line := range strings.Split(nextSteps(), "\n") {
			lines = append(lines, " "+clip(line, width-1))
		}
	}

	return panel{
		title:   "result",
		hint:    m.elapsed(),
		lines:   lines,
		width:   l.content,
		height:  cardHeight(l.body, lines),
		framed:  l.framed,
		focused: true,
		phase:   m.phase(),

		border: lipgloss.DoubleBorder(),
		custom: true,
	}.render()
}

func nextSteps() string {
	return lipglosslist.New(
		theme.MutedS.Render("press ")+theme.Key.Render("v")+theme.MutedS.Render(" to open a test stream"),
		theme.MutedS.Render("quit and reopen Helium so it loads the new CDM"),
		theme.MutedS.Render("press ")+theme.Key.Render("d")+theme.MutedS.Render(" if playback still fails"),
	).
		Enumerator(func(lipglosslist.Items, int) string { return "› " }).
		EnumeratorStyle(theme.FaintS).
		ItemStyle(theme.MutedS).
		String()
}

func restartAdvice(running proc.Status) string {
	if !running.Running {
		return theme.MutedS.Render("Restart Helium for the change to take effect.")
	}

	if running.Matches {
		return theme.Warn.Render("Helium is running: quit and reopen it to load the new CDM.")
	}

	return theme.MutedS.Render("Restart Helium for the change to take effect.")
}

func originNote(origin browser.Origin) string {
	switch origin {
	case browser.OriginBundled:
		return "shipped inside the browser install"
	case browser.OriginComponentUpdater:
		return "downloaded by the browser component updater"
	}

	return "provided on the command line"
}

func (m *Model) renderFailed(l layout) string {
	width := l.text(l.content)

	lines := []string{"", " " + theme.Bad.Bold(true).Render("✗") + "  " + theme.Heading.Render(clip(errText(m.err), width-4))}

	hints := hintsFor(m.err)
	if len(hints) > 0 {
		lines = append(lines, "")
		for _, hint := range hints {
			for i, line := range wrapText(hint, width-6) {
				if i == 0 {
					lines = append(lines, " "+theme.MutedS.Render("· "+line))
					continue
				}
				lines = append(lines, "   "+theme.MutedS.Render(line))
			}
		}
	}

	lines = append(lines, "", " "+theme.MutedS.Render("press r to rescan or q to quit"))

	return panel{
		title:   "error",
		hint:    "nothing was changed",
		lines:   lines,
		width:   l.content,
		height:  cardHeight(l.body, lines),
		framed:  l.framed,
		focused: true,
		phase:   m.phase(),
	}.render()
}

func labeled(label, value string, width int) []string {
	const col = 9

	if width < 24 {
		out := []string{" " + theme.Label.Render(label)}
		for _, line := range wrapValue(value, max(width-2, 6)) {
			out = append(out, " "+theme.Value.Render(line))
		}

		return out
	}

	valueStyle := theme.Value
	if strings.HasPrefix(value, "/") && lipgloss.Width(value) <= width-col-2 {
		valueStyle = theme.Link.Hyperlink("file://" + value)
	}

	var out []string
	for i, line := range wrapValue(value, max(width-col-2, 10)) {
		if i == 0 {
			out = append(out, " "+theme.Label.Render(padRight(label, col))+" "+valueStyle.Render(line))
			continue
		}
		out = append(out, " "+strings.Repeat(" ", col)+" "+theme.Value.Render(line))
	}

	return out
}

func wrapValue(value string, width int) []string {
	if strings.ContainsAny(value, `/:\\`) {
		return wrapPath(value, width)
	}

	return wrapText(value, width)
}

func sourceLabel(source fixer.Source) string {
	label := source.Name

	if source.Version != "" && !strings.Contains(label, source.Version) {
		label += " · CDM " + source.Version
	}

	return label
}
