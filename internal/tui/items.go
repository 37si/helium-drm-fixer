package tui

import (
	"fmt"
	"io"
	"strconv"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"hdf/internal/browser"
)

type sourceItem struct {
	detection browser.Detection
	fresh     bool
}

func (i sourceItem) Title() string {
	if i.fresh {
		return "Fresh download"
	}

	return i.detection.Name
}

func (i sourceItem) FilterValue() string {
	if i.fresh {
		return "fresh download mozilla widevine"
	}

	return i.detection.Name
}

func (i sourceItem) usable() bool { return i.fresh || i.detection.Available() }

func (i sourceItem) version() string {
	if i.fresh {
		return ""
	}

	if !i.detection.Available() || i.detection.CDM.Version == "" {
		return ""
	}

	return i.detection.CDM.Version
}

type sourceDelegate struct {
	hover *string
}

func newSourceDelegate(hover *string) sourceDelegate { return sourceDelegate{hover: hover} }

func rowName(index int) string { return "row:" + strconv.Itoa(index) }

func (sourceDelegate) Height() int                         { return 1 }
func (sourceDelegate) Spacing() int                        { return 0 }
func (sourceDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (d sourceDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	src, ok := item.(sourceItem)
	if !ok {
		return
	}

	width := m.Width()
	selected := index == m.Index()

	cursor, dot := "  ", theme.FaintS.Render("○")
	switch {
	case src.fresh:
		cursor, dot = theme.Accent.Render("▌ "), theme.Accent.Render("◆")
		if !selected {
			cursor = "  "
		}
	case !src.detection.Available():
		dot = theme.FaintS.Render("○")
	case selected:
		cursor, dot = theme.Accent.Render("▌ "), theme.Good.Render("●")
	default:
		dot = theme.Good.Render("●")
	}

	version := src.version()
	versionWidth := 0
	if width >= 34 {
		versionWidth = 12
	}

	nameWidth := max(width-lipgloss.Width(cursor)-3-versionWidth, 8)
	name := padRight(clip(src.Title(), nameWidth), nameWidth)

	switch {
	case src.fresh && selected:
		name = theme.Accent.Bold(true).Render(name)
	case src.fresh:
		name = theme.Accent.Render(name)
	case !src.detection.Available():
		name = theme.FaintS.Render(name)
	case selected:
		name = theme.Accent.Bold(true).Render(name)
	default:
		name = theme.Value.Render(name)
	}

	row := cursor + dot + " " + name
	if versionWidth > 0 {
		label, style := "—", theme.FaintS
		switch {
		case src.fresh:
			label, style = "newest", theme.Accent
		case version != "":
			label, style = version, theme.MutedS
		}
		row += " " + style.Render(padLeft(label, versionWidth-1))
	}

	if d.hover != nil && *d.hover == rowName(index) {
		fmt.Fprint(w, band(padRight(row, width), theme.Surface))
		return
	}

	fmt.Fprint(w, row)
}

func (m *Model) listView() string {
	key := listKey{
		index:  m.list.Index(),
		hover:  m.hover,
		width:  m.list.Width(),
		height: m.list.Height(),
		items:  len(m.list.Items()),
		filter: m.list.FilterState(),
		typed:  m.list.FilterInput.Value(),
	}

	if key == m.listCacheKey {
		return m.listCache
	}

	m.listCache, m.listCacheKey = m.list.View(), key

	return m.listCache
}

func (m *Model) freshDetail(width, height int) []string {
	block := []string{
		"",
		" " + theme.Heading.Render("Fresh download") + " " + theme.Accent.Render("◆"),
		" " + hairlineAt(max(width-1, 1), m.phase()),
		"",
	}

	block = append(block, labeled("source", "aus5.mozilla.org, the service Firefox uses", width)...)
	block = append(block, labeled("version", "whatever Mozilla serves right now", width)...)
	block = append(block, labeled("check", "SHA-512 from the update document", width)...)
	block = append(block, labeled("fallback", "a browser copy if the download fails", width)...)
	block = append(block, "")

	for _, line := range wrapText("No Chrome or Firefox install is touched.", max(width-2, 6)) {
		block = append(block, " "+theme.FaintS.Render(line))
	}

	footer := []string{
		" " + rule(width-1),
		" " + keycap("↵", "downloads it now"),
	}

	return pinFooter(block, footer, max(height-frameHeight, 0))
}
