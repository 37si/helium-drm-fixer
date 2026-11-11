package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

type ink struct {
	ansi      color.Color
	ansi256   color.Color
	truecolor color.Color
}

func rgb(hex string) color.Color { return lipgloss.Color(hex) }

func newInk(ansi, ansi256, truecolor string) ink {
	return ink{ansi: rgb(ansi), ansi256: rgb(ansi256), truecolor: rgb(truecolor)}
}

var (
	inkPrimary   = newInk("#5F87FF", "#87AFFF", "#A9C9FF")
	inkSecondary = newInk("#FF87D7", "#FFAFD7", "#FFBBEC")
	inkTertiary  = newInk("#FFAF87", "#FFD7AF", "#FFC3A0")
	inkSuccess   = newInk("#5FD7AF", "#87D7AF", "#7FE3BD")
	inkDanger    = newInk("#FF5F87", "#FF8787", "#FF9AA8")
	inkWarning   = newInk("#FFAF5F", "#FFD787", "#FFCF8F")
	inkMuted     = newInk("#8787AF", "#8787AF", "#8B93AE")
	inkFaint     = newInk("#585858", "#585858", "#454D68")
	inkText      = newInk("#E4E4E4", "#E4E4E4", "#E6E9F4")
	inkInk       = newInk("#121212", "#121212", "#0B0D12")
)

type Theme struct {
	Profile   colorprofile.Profile
	Dark      bool
	Primary   color.Color
	Secondary color.Color
	Tertiary  color.Color
	Success   color.Color
	Danger    color.Color
	Warning   color.Color
	Muted     color.Color
	Faint     color.Color
	Text      color.Color
	Ink       color.Color
	Surface   color.Color
	Accent    lipgloss.Style
	Good      lipgloss.Style
	Warn      lipgloss.Style
	Bad       lipgloss.Style
	MutedS    lipgloss.Style
	FaintS    lipgloss.Style
	Value     lipgloss.Style
	Label     lipgloss.Style
	Key       lipgloss.Style
	Heading   lipgloss.Style
	Title     lipgloss.Style
	TitleHot  lipgloss.Style
	Hint      lipgloss.Style
	Keycap    lipgloss.Style
	KeycapHi  lipgloss.Style
	Link      lipgloss.Style
	complete  lipgloss.CompleteFunc
}

var (
	base   = newBaseTheme()
	theme  = base
	dimmed bool
)

func newBaseTheme() *Theme {
	t := &Theme{}
	t.build(colorprofile.TrueColor, true, nil)
	return t
}

func Configure(profile colorprofile.Profile, dark bool, background color.Color) {
	base = &Theme{}
	base.build(profile, dark, background)

	theme = base
	dimmed = false
	clearCaches()
}

func Current() *Theme { return theme }

func (t *Theme) build(profile colorprofile.Profile, dark bool, background color.Color) {
	t.Profile = profile
	t.Dark = dark
	t.complete = lipgloss.Complete(profile)

	switch {
	case background != nil:
		t.Ink = background
	case dark:
		t.Ink = t.pick(inkInk)
	default:
		t.Ink = rgb("#F7F8FC")
	}

	t.Primary = t.pick(inkPrimary)
	t.Secondary = t.pick(inkSecondary)
	t.Tertiary = t.pick(inkTertiary)
	t.Success = t.pick(inkSuccess)
	t.Danger = t.pick(inkDanger)
	t.Warning = t.pick(inkWarning)
	t.Muted = t.pick(inkMuted)
	t.Faint = t.pick(inkFaint)
	t.Text = t.pick(inkText)

	if !dark {
		t.Primary = lipgloss.Darken(t.Primary, 0.42)
		t.Secondary = lipgloss.Darken(t.Secondary, 0.45)
		t.Tertiary = lipgloss.Darken(t.Tertiary, 0.45)
		t.Success = lipgloss.Darken(t.Success, 0.38)
		t.Danger = lipgloss.Darken(t.Danger, 0.32)
		t.Warning = lipgloss.Darken(t.Warning, 0.40)
		t.Muted = lipgloss.Darken(t.Muted, 0.30)
		t.Text = rgb("#1A1D28")
	}

	t.Surface = blend(t.Ink, t.Primary, 0.22)
	t.setStyles()
}

func (t *Theme) setStyles() {
	t.Accent = lipgloss.NewStyle().Foreground(t.Primary)
	t.Good = lipgloss.NewStyle().Foreground(t.Success)
	t.Warn = lipgloss.NewStyle().Foreground(t.Warning)
	t.Bad = lipgloss.NewStyle().Foreground(t.Danger)
	t.MutedS = lipgloss.NewStyle().Foreground(t.Muted)
	t.FaintS = lipgloss.NewStyle().Foreground(t.Faint)
	t.Value = lipgloss.NewStyle().Foreground(t.Text)
	t.Label = lipgloss.NewStyle().Foreground(t.Muted).Faint(true)
	t.Key = lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	t.Heading = lipgloss.NewStyle().Foreground(t.Text).Bold(true)
	t.Title = lipgloss.NewStyle().Foreground(t.Muted)
	t.TitleHot = lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	t.Hint = lipgloss.NewStyle().Foreground(t.Faint)
	t.Keycap = lipgloss.NewStyle().Foreground(t.Muted)
	t.KeycapHi = lipgloss.NewStyle().Foreground(t.Ink).Background(t.Primary).Bold(true)
	t.Link = lipgloss.NewStyle().Foreground(t.Text).Underline(true).UnderlineColor(t.Faint)
}

func (t *Theme) Frame(focused bool, step int) lipgloss.Style {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Faint).
		Padding(0, 1)

	if !focused {
		return style
	}

	return style.
		BorderForegroundBlend(t.Primary, t.Secondary, t.Tertiary, t.Primary).
		BorderForegroundBlendOffset(step)
}

func (t *Theme) pick(i ink) color.Color {
	return t.complete(i.ansi, i.ansi256, i.truecolor)
}

func SetDimmed(on bool) {
	if on == dimmed {
		return
	}

	dimmed = on
	clearCaches()

	if on {
		theme = base.dimmed()
		return
	}

	theme = base
}

func (t *Theme) dimmed() *Theme {
	faded := *t

	faded.Primary = blend(t.Primary, t.Ink, 0.55)
	faded.Secondary = blend(t.Secondary, t.Ink, 0.55)
	faded.Tertiary = blend(t.Tertiary, t.Ink, 0.55)
	faded.Success = blend(t.Success, t.Ink, 0.45)
	faded.Danger = blend(t.Danger, t.Ink, 0.45)
	faded.Warning = blend(t.Warning, t.Ink, 0.45)
	faded.Text = blend(t.Text, t.Ink, 0.35)
	faded.Muted = blend(t.Muted, t.Ink, 0.30)
	faded.Faint = blend(t.Faint, t.Ink, 0.25)
	faded.Surface = blend(t.Surface, t.Ink, 0.40)
	faded.setStyles()

	return &faded
}

func blend(a, b color.Color, f float64) color.Color {
	switch {
	case f <= 0:
		return a
	case f >= 1:
		return b
	}

	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()

	mix := func(x, y uint32) uint8 {
		return uint8(float64(x>>8)*(1-f) + float64(y>>8)*f)
	}

	return color.RGBA{R: mix(ar, br), G: mix(ag, bg), B: mix(ab, bb), A: 255}
}

func shade(c color.Color, f float64) color.Color { return blend(c, theme.Ink, 1-f) }
