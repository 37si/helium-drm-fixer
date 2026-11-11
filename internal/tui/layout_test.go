package tui

import (
	"io"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/log/v2"
	"github.com/charmbracelet/x/ansi"

	"hdf/internal/browser"
)

func sizes() []struct{ w, h int } {
	var out []struct{ w, h int }

	for _, w := range []int{8, 10, 16, 20, 24, 30, 36, 40, 44, 50, 58, 66, 74, 76, 80, 96, 110, 120, 140, 200, 240, 320} {
		for _, h := range []int{1, 2, 3, 4, 5, 8, 10, 12, 14, 18, 24, 30, 40, 60, 80, 100} {
			out = append(out, struct{ w, h int }{w, h})
		}
	}

	return out
}

func sizedModel(t *testing.T, w, h int) *Model {
	t.Helper()

	m := New(t.Context(), Options{Version: "test"}, log.New(io.Discard))
	model, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	m = model.(*Model)

	model, _ = m.Update(scanDoneMsg{detections: []browser.Detection{
		chromeDetection(true),
		{Browser: browser.Browser{ID: browser.Brave, Name: "Brave", Priority: 30}, Executable: "/usr/bin/brave-browser"},
	}})

	m = model.(*Model)
	m.plan = samplePlan()
	m.report = sampleReport()
	m.reportAt = time.Now()
	m.checks = sampleChecks()
	m.watchEvents = sampleEvents()

	return m
}

func TestFrameFitsEverySize(t *testing.T) {
	for _, size := range sizes() {
		m := sizedModel(t, size.w, size.h)

		for _, screen := range allScreens() {
			m.screen = screen
			if screen == screenHelp {
				m.history = append(m.history[:0], screenPicking)
				m.viewport.SetContent(m.renderHelp())
			}

			frame := m.render()
			lines := strings.Split(frame, "\n")

			if len(lines) > size.h {
				t.Fatalf("%dx%d screen %d: %d rows", size.w, size.h, screen, len(lines))
			}

			for i, line := range lines {
				if got := lipgloss.Width(line); got > size.w {
					t.Fatalf("%dx%d screen %d: row %d is %d cells wide: %q", size.w, size.h, screen, i, got, line)
				}
			}
		}
	}
}

func allScreens() []screen {
	return []screen{
		screenScanning,
		screenPicking,
		screenReview,
		screenApplying,
		screenDone,
		screenFailed,
		screenHelp,
		screenStatus,
		screenDoctor,
		screenWatch,
		screenClear,
	}
}

func TestFrameFillsEverySize(t *testing.T) {
	for _, size := range sizes() {
		m := sizedModel(t, size.w, size.h)
		m.screen = screenPicking

		if got := len(strings.Split(m.render(), "\n")); got != size.h {
			t.Fatalf("%dx%d: %d rows, want %d", size.w, size.h, got, size.h)
		}
	}
}

func TestTinyTerminalDropsChrome(t *testing.T) {
	m := sizedModel(t, 20, 5)
	l := m.layout()

	if l.header || l.footer || l.framed {
		t.Fatalf("20x5 kept decoration: header=%v footer=%v framed=%v", l.header, l.footer, l.framed)
	}
	if l.detail {
		t.Fatal("20x5 kept the detail pane")
	}
}

func TestLayoutGrowsWithTerminal(t *testing.T) {
	small := sizedModel(t, 60, 20).layout()
	large := sizedModel(t, 120, 40).layout()

	if small.content >= large.content {
		t.Fatalf("content did not grow: %d vs %d", small.content, large.content)
	}
	if small.body >= large.body {
		t.Fatalf("body did not grow: %d vs %d", small.body, large.body)
	}
	if !large.header || !large.footer || !large.detail {
		t.Fatal("a large terminal should show every region")
	}
}

func TestWideTerminalIsCapped(t *testing.T) {
	l := sizedModel(t, 400, 40).layout()
	if l.content > maxContent {
		t.Fatalf("content = %d, want at most %d", l.content, maxContent)
	}
}

func TestBodyNeverExceedsHeight(t *testing.T) {
	for _, size := range sizes() {
		l := sizedModel(t, size.w, size.h).layout()
		if l.body > size.h {
			t.Fatalf("%dx%d: body = %d", size.w, size.h, l.body)
		}
		if l.body < 1 {
			t.Fatalf("%dx%d: body = %d", size.w, size.h, l.body)
		}
	}
}

func TestEveryScreenFillsTheTerminal(t *testing.T) {
	for _, size := range sizes() {
		for _, screen := range allScreens() {
			m := sizedModel(t, size.w, size.h)
			m.screen = screen

			if screen == screenHelp {
				m.viewport.SetContent(m.renderHelp())
			}

			if got := len(strings.Split(m.render(), "\n")); got != size.h {
				t.Fatalf("%dx%d screen %d: %d rows, want %d", size.w, size.h, screen, got, size.h)
			}
		}
	}
}

func TestContentIsCentred(t *testing.T) {
	for _, size := range []struct{ w, h int }{{200, 40}, {140, 30}, {120, 40}, {80, 24}, {40, 20}} {
		l := sizedModel(t, size.w, size.h).layout()

		left := l.offset
		right := size.w - l.content - l.offset
		if diff := left - right; diff < -1 || diff > 1 {
			t.Fatalf("%dx%d: %d columns left, %d right", size.w, size.h, left, right)
		}
	}
}

func TestWideFrameIsCentred(t *testing.T) {
	m := sizedModel(t, 200, 40)
	m.screen = screenPicking

	for i, line := range strings.Split(m.render(), "\n") {
		text := strings.TrimLeft(ansi.Strip(line), " ")
		if text == "" || !strings.ContainsRune("╭│╰─", rune(text[0])) {
			continue
		}

		width := ansi.StringWidth(line)
		left := width - ansi.StringWidth(strings.TrimLeft(line, " "))
		right := width - ansi.StringWidth(strings.TrimRight(line, " "))
		if diff := left - right; diff < -1 || diff > 1 {
			t.Fatalf("row %d: %d columns left, %d right", i, left, right)
		}
	}
}

func TestShortBodyIsCentred(t *testing.T) {
	m := sizedModel(t, 100, 40)
	m.screen = screenDone

	lines := strings.Split(m.render(), "\n")

	first, last := -1, -1
	for i := 4; i < len(lines)-2; i++ {
		if strings.TrimSpace(ansi.Strip(lines[i])) == "" {
			continue
		}
		if first < 0 {
			first = i
		}
		last = i
	}

	if first < 0 {
		t.Fatal("nothing rendered")
	}
	if first-4 < 3 {
		t.Fatalf("the card starts at row %d, too close to the header", first)
	}
	if len(lines)-2-last < 3 {
		t.Fatalf("the card ends at row %d, too close to the footer", last)
	}
}
