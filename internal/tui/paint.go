package tui

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	animSteps = 12
	flashLife = 12
)

func stepOf(phase float64) int {
	if phase <= 0 {
		return 0
	}

	return int(math.Round(phase*animSteps)) % animSteps
}

func clearCaches() {
	clear(glyphCache)
	clear(stopCache)
	clear(hairCache)
	clear(boxCache)
}

type glyphKey struct {
	c    color.Color
	r    rune
	bold bool
}

var glyphCache = make(map[glyphKey]string, 512)

func glyph(c color.Color, r rune, bold bool) string {
	key := glyphKey{c: c, r: r, bold: bold}
	if s, ok := glyphCache[key]; ok {
		return s
	}

	if len(glyphCache) > 4096 {
		clear(glyphCache)
	}

	style := lipgloss.NewStyle().Foreground(c)
	if bold {
		style = style.Bold(true)
	}

	s := style.Render(string(r))
	glyphCache[key] = s

	return s
}

type stopKey struct {
	n    int
	step int
}

var (
	stopCache = make(map[stopKey][]color.Color, 64)
	hairCache = make(map[stopKey]string, 64)
)

func ramp(n int, phase float64) []color.Color {
	if n <= 0 {
		return nil
	}

	key := stopKey{n: n, step: stepOf(phase)}
	if stops, ok := stopCache[key]; ok {
		return stops
	}

	stops := lipgloss.Blend1D(n, theme.Primary, theme.Secondary, theme.Tertiary)

	if key.step > 0 {
		span := float64(n)
		centre := float64(key.step) / animSteps * span
		sigma := math.Max(span/7, 1.5)

		for i := range stops {
			distance := math.Abs(float64(i) - centre)
			if rest := span - distance; rest < distance {
				distance = rest
			}

			glow := math.Exp(-(distance * distance) / (2 * sigma * sigma))
			stops[i] = blend(stops[i], theme.Text, 0.55*glow)
		}
	}

	if len(stopCache) > 256 {
		clear(stopCache)
	}
	stopCache[key] = stops

	return stops
}

func gradient(text string, stops []color.Color, bold bool) string {
	if text == "" || len(stops) == 0 {
		return text
	}

	var b strings.Builder
	b.Grow(len(text) * 8)

	for i, r := range text {
		b.WriteString(glyph(stops[min(i, len(stops)-1)], r, bold))
	}

	return b.String()
}

func wordmarkAt(text string, phase float64) string {
	runes := []rune(text)

	return gradient(text, ramp(len(runes), phase), true)
}

func hairline(width int) string { return hairlineAt(width, 0) }

func hairlineAt(width int, phase float64) string {
	if width <= 0 {
		return ""
	}

	key := stopKey{n: width, step: stepOf(phase)}
	if s, ok := hairCache[key]; ok {
		return s
	}

	stops := ramp(width, phase)
	for i := range stops {
		stops[i] = shade(stops[i], 0.58)
	}

	s := gradient(strings.Repeat("─", width), stops, false)

	if len(hairCache) > 64 {
		clear(hairCache)
	}
	hairCache[key] = s

	return s
}

func rule(width int) string {
	if width <= 0 {
		return ""
	}

	return theme.FaintS.Render(strings.Repeat("─", width))
}

func band(row string, bg color.Color) string {
	r, g, b, _ := bg.RGBA()
	seq := fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r>>8, g>>8, b>>8)

	row = strings.ReplaceAll(row, "\x1b[m", "\x1b[m"+seq)
	row = strings.ReplaceAll(row, "\x1b[0m", "\x1b[0m"+seq)

	return seq + row + "\x1b[m"
}

func keycap(key, label string) string {
	if key == "" {
		return theme.MutedS.Render(label)
	}

	return theme.Key.Render(key) + " " + theme.MutedS.Render(label)
}

func hotKeycap(key, label string) string {
	if key == "" {
		return theme.MutedS.Render(label)
	}

	return theme.KeycapHi.Render(key) + " " + theme.Value.Render(label)
}

func padRight(s string, width int) string {
	gap := width - lipgloss.Width(s)
	if gap <= 0 {
		return s
	}

	return s + strings.Repeat(" ", gap)
}

func padLeft(s string, width int) string {
	gap := width - lipgloss.Width(s)
	if gap <= 0 {
		return s
	}

	return strings.Repeat(" ", gap) + s
}

func clip(s string, width int) string {
	if width <= 0 {
		return ""
	}

	return ansi.Truncate(s, width, "…")
}

func truncate(s string, limit int) string {
	if limit <= 1 {
		return s
	}

	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}

	return "…" + string(runes[len(runes)-limit+1:])
}

func wrapText(text string, width int) []string {
	if width < 8 || lipgloss.Width(text) <= width {
		return []string{text}
	}

	return strings.Split(lipgloss.Wrap(text, width, ""), "\n")
}

func wrapPath(path string, width int) []string {
	if width < 8 || lipgloss.Width(path) <= width {
		return []string{path}
	}

	var lines []string
	line := ""

	for _, part := range strings.SplitAfter(path, "/") {
		if lipgloss.Width(line+part) > width && line != "" {
			lines = append(lines, line)
			line = part
			continue
		}
		line += part
	}

	if line != "" {
		lines = append(lines, line)
	}

	return splitLong(lines, width)
}

func splitLong(lines []string, width int) []string {
	out := make([]string, 0, len(lines))

	for _, line := range lines {
		runes := []rune(line)
		for len(runes) > width {
			out = append(out, string(runes[:width]))
			runes = runes[width:]
		}

		out = append(out, string(runes))
	}

	return out
}

func splice(s string, at, width int, insert string) string {
	if at < 0 || width < 0 {
		return s
	}

	return ansi.Cut(s, 0, at) + insert + ansi.TruncateLeft(s, at+width, "")
}

func indent(block string, n int) string {
	if n <= 0 {
		return block
	}

	pad := strings.Repeat(" ", n)
	lines := strings.Split(block, "\n")
	for i := range lines {
		lines[i] = pad + lines[i]
	}

	return strings.Join(lines, "\n")
}
