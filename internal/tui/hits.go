package tui

import (
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

type hit struct {
	x, y, w int
	name    string
	run     func(*Model) tea.Cmd
}

func (h hit) contains(x, y int) bool {
	return y == h.y && x >= h.x && x < h.x+h.w
}

func (m *Model) addHit(x, y, w int, name string, run func(*Model) tea.Cmd) {
	if w <= 0 || y < 0 || x < 0 {
		return
	}

	m.hits = append(m.hits, hit{x: x, y: y, w: w, name: name, run: run})
}

func (m *Model) hitAt(x, y int) (hit, bool) {
	for i := len(m.hits) - 1; i >= 0; i-- {
		if m.hits[i].contains(x, y) {
			return m.hits[i], true
		}
	}

	return hit{}, false
}

func (m *Model) onMouse(msg tea.MouseMsg) tea.Cmd {
	switch e := msg.(type) {
	case tea.MouseClickMsg:
		if e.Button != tea.MouseLeft {
			return nil
		}

		found, ok := m.hitAt(e.X, e.Y)
		if !ok || found.run == nil {
			return nil
		}

		return found.run(m)

	case tea.MouseMotionMsg:
		name := ""
		if found, ok := m.hitAt(e.X, e.Y); ok {
			name = found.name
		}

		m.hover = name

		return nil

	case tea.MouseWheelMsg:
		switch e.Button {
		case tea.MouseWheelUp:
			return m.wheelScroll(-1)
		case tea.MouseWheelDown:
			return m.wheelScroll(1)
		}
	}

	return nil
}

func (m *Model) wheelScroll(direction int) tea.Cmd {
	switch m.screen {
	case screenPicking:
		if m.list.FilterState() != list.Unfiltered {
			return nil
		}

		if direction < 0 {
			m.list.CursorUp()
		} else {
			m.list.CursorDown()
		}

	case screenHelp:
		m.viewport.SetYOffset(m.viewport.YOffset() + direction*3)

	case screenDoctor:
		m.scroll = max(m.scroll+direction*3, 0)
		m.scroll = min(m.scroll, m.maxScroll())

	case screenWatch:
		m.watchScroll = max(m.watchScroll+direction*3, 0)
		m.watchScroll = min(m.watchScroll, m.maxWatchScroll())
	}

	return nil
}

func press(key string) func(*Model) tea.Cmd {
	return func(m *Model) tea.Cmd {
		msg, ok := keyPress(key)
		if !ok {
			return nil
		}

		_, cmd := m.onKey(msg)

		return cmd
	}
}

func keyPress(key string) (tea.KeyPressMsg, bool) {
	switch key {
	case "↵":
		return tea.KeyPressMsg{Code: tea.KeyEnter}, true
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}, true
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}, true
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}, true
	case "↑↓", "+/-", "":
		return tea.KeyPressMsg{}, false
	}

	runes := []rune(key)
	if len(runes) != 1 {
		return tea.KeyPressMsg{}, false
	}

	return tea.KeyPressMsg{Code: runes[0], Text: key}, true
}
