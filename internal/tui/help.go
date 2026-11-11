package tui

import (
	_ "embed"
	"image/color"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/ansi"
	"github.com/charmbracelet/colorprofile"
)

//go:embed help.md
var helpMarkdown string

func (m *Model) openHelp() {
	m.show(screenHelp)
	m.viewport.SetContent(m.renderHelp())
	m.viewport.GotoTop()
}

func (m *Model) renderHelp() string {
	width := clamp(m.layout().text(m.layout().content)-2, 16, 92)

	if m.helpCache != "" && m.helpWidth == width {
		return m.helpCache
	}

	renderer, err := glamour.NewTermRenderer(
		glamour.WithStyles(helpStyles()),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return helpMarkdown
	}

	out, err := renderer.Render(helpMarkdown + m.capabilitySection())
	if err != nil {
		return helpMarkdown
	}

	m.helpCache, m.helpWidth = out, width
	return out
}

func (m *Model) capabilitySection() string {
	mouse := "click, wheel and hover"
	if m.opts.NoMouse {
		mouse = "off, `--no-mouse` is set"
	}

	keyboard := "plain escape sequences"
	switch {
	case m.caps.kitty && m.caps.release:
		keyboard = "kitty protocol, with key release events"
	case m.caps.kitty:
		keyboard = "kitty protocol"
	}

	colour := "256 colours"
	if theme.Profile >= colorprofile.TrueColor {
		colour = "truecolor"
	}

	return "\n## This terminal\n\n" +
		"- **colour** " + colour + "\n" +
		"- **mouse** " + mouse + "\n" +
		"- **keyboard** " + keyboard + "\n" +
		"- **focus** the interface dims and the animation pauses when the window loses focus\n" +
		"- **progress** reported to the tab or taskbar, and to the shell where the terminal asks\n"
}

func helpStyles() ansi.StyleConfig {
	text := hex(theme.Text)
	accent := hex(theme.Primary)
	second := hex(theme.Secondary)
	muted := hex(theme.Muted)

	colour := func(s string) *string { return &s }
	str := func(s string) *string { return &s }
	flag := func(b bool) *bool { return &b }
	size := func(n uint) *uint { return &n }

	return ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:       colour(text),
				BlockPrefix: "\n",
				BlockSuffix: "\n",
			},
			Margin: size(1),
		},
		Paragraph: ansi.StyleBlock{},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:       colour(accent),
				Bold:        flag(true),
				BlockSuffix: "\n",
			},
		},
		H1: ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{
			Color: colour(accent),
			Bold:  flag(true),
		}},
		H2: ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{
			Color: colour(accent),
			Bold:  flag(true),
		}},
		H3:     ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{Color: colour(second)}},
		Strong: ansi.StylePrimitive{Color: colour(accent), Bold: flag(true)},
		Emph:   ansi.StylePrimitive{Color: colour(second), Italic: flag(true)},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{Color: colour(second), Prefix: "", Suffix: ""},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{Color: colour(second)}},
		},
		List:        ansi.StyleList{LevelIndent: 2},
		Item:        ansi.StylePrimitive{BlockPrefix: "• ", Color: colour(muted)},
		Enumeration: ansi.StylePrimitive{BlockPrefix: ". ", Color: colour(muted)},
		Table: ansi.StyleTable{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{Color: colour(text)},
				Margin:         size(1),
			},
			CenterSeparator: str("┼"),
			ColumnSeparator: str("│"),
			RowSeparator:    str("─"),
		},
		Link: ansi.StylePrimitive{
			Color:     colour(accent),
			Underline: flag(true),
		},
		LinkText: ansi.StylePrimitive{Color: colour(accent), Bold: flag(true)},
		HorizontalRule: ansi.StylePrimitive{
			Color:  colour(muted),
			Format: "\n--------\n",
		},
	}
}

func hex(c color.Color) string {
	r, g, b, _ := c.RGBA()

	const digits = "0123456789abcdef"
	out := []byte{'#'}

	for _, v := range []uint32{r >> 8, g >> 8, b >> 8} {
		out = append(out, digits[(v>>4)&0xf], digits[v&0xf])
	}

	return string(out)
}
