package tui

import (
	"hash/fnv"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const vCenter = 1

type panel struct {
	title   string
	hint    string
	lines   []string
	width   int
	height  int
	framed  bool
	focused bool
	valign  int
	phase   float64
	border  lipgloss.Border
	custom  bool
}

const frameWidth = 4

const frameHeight = 2

func (p panel) render() string {
	if !p.framed {
		return p.plain()
	}

	width := max(p.width, 12)
	height := max(p.height, 3)

	key := p.cacheKey(width, height)
	if cached, ok := boxCache[key]; ok {
		return cached
	}

	body := p.body(width-frameWidth, height-frameHeight)

	style := theme.Frame(p.focused, stepOf(p.phase))
	if p.custom {
		style = style.Border(p.border)
	}

	box := style.
		Width(width).
		Height(height).
		Render(body)

	out := p.decorate(box, width)

	if len(boxCache) > 192 {
		clear(boxCache)
	}
	boxCache[key] = out

	return out
}

func (p panel) body(width, height int) string {
	width = max(width, 4)
	height = max(height, 1)

	rows := make([]string, 0, height)
	lead := p.lead(height)

	for i := 0; i < height; i++ {
		line := ""
		if j := i - lead; j >= 0 && j < len(p.lines) {
			line = p.lines[j]
		}
		rows = append(rows, padRight(clip(line, width), width))
	}

	return strings.Join(rows, "\n")
}

func (p panel) lead(rows int) int {
	if p.valign != vCenter || len(p.lines) >= rows {
		return 0
	}

	return (rows - len(p.lines)) / 2
}

func (p panel) decorate(box string, width int) string {
	title := clip(p.title, max(width-14, 4))
	if title == "" {
		return box
	}

	top, rest, ok := strings.Cut(box, "\n")
	if !ok {
		return box
	}

	style := theme.Title
	if p.focused {
		style = theme.TitleHot
	}

	label := " " + style.Render(title) + " "
	top = splice(top, 2, lipgloss.Width(label), label)

	if p.hint != "" {
		room := width - 2 - lipgloss.Width(label) - 6
		if room >= 8 {
			hint := " " + theme.Hint.Render(clip(p.hint, room)) + " "
			at := width - 2 - lipgloss.Width(hint)
			top = splice(top, at, lipgloss.Width(hint), hint)
		}
	}

	return top + "\n" + rest
}

func (p panel) cacheKey(width, height int) string {
	hash := fnv.New64a()

	for _, line := range p.lines {
		hash.Write([]byte(line))
		hash.Write([]byte{0})
	}

	return strings.Join([]string{
		p.title,
		p.hint,
		strconv.Itoa(width),
		strconv.Itoa(height),
		strconv.Itoa(p.valign),
		strconv.Itoa(stepOf(p.phase)),
		strconv.FormatBool(p.framed),
		strconv.FormatBool(p.focused),
		strconv.FormatUint(hash.Sum64()&0xffffffff, 10),
	}, "\x00")
}

var boxCache = make(map[string]string, 192)

func (p panel) plain() string {
	width := max(p.width, 1)
	height := max(p.height, 1)

	rows := make([]string, 0, height)
	lead := p.lead(height)

	for i := 0; i < height; i++ {
		line := ""
		if j := i - lead; j >= 0 && j < len(p.lines) {
			line = p.lines[j]
		}
		rows = append(rows, padRight(clip(line, width), width))
	}

	return strings.Join(rows, "\n")
}

func fitHeight(maxHeight int, lines []string) int {
	return clamp(len(lines)+frameHeight, 3, maxHeight)
}

func cardHeight(body int, lines []string) int {
	need := len(lines) + frameHeight
	if body-need > 6 {
		return max(need+2, 3)
	}

	return max(body, 3)
}

func fitLines(lines []string, n int) []string {
	if n <= 0 {
		return nil
	}
	if len(lines) > n {
		return lines[:n]
	}

	return lines
}

func pinFooter(block, footer []string, room int) []string {
	if room <= 0 {
		return block
	}

	if len(footer) > room {
		footer = footer[len(footer)-room:]
	}

	body := room - len(footer)
	if len(block) > body {
		block = fitLines(block, body)
	}

	out := make([]string, 0, room)
	out = append(out, block...)

	for len(out) < body {
		out = append(out, "")
	}

	return append(out, footer...)
}

func blank(line string) bool {
	return strings.TrimSpace(ansi.Strip(line)) == ""
}
