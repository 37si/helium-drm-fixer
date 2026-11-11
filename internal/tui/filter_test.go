package tui

import (
	"strings"
	"testing"

	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func feed(t *testing.T, m *Model, msg tea.Msg) *Model {
	t.Helper()

	model, cmd := m.Update(msg)
	m = model.(*Model)

	for _, next := range run(t, cmd) {
		model, _ = m.Update(next)
		m = model.(*Model)
	}

	return m
}

func run(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()

	if cmd == nil {
		return nil
	}

	out, ok := call(cmd)
	if !ok {
		return nil
	}

	batch, ok := out.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{out}
	}

	var msgs []tea.Msg
	for _, inner := range batch {
		if next, ok := call(inner); ok {
			msgs = append(msgs, next)
		}
	}

	return msgs
}

func call(cmd tea.Cmd) (tea.Msg, bool) {
	if cmd == nil {
		return nil, false
	}

	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()

	select {
	case msg := <-done:
		return msg, msg != nil
	case <-time.After(50 * time.Millisecond):
		return nil, false
	}
}

func TestTheListCacheNoticesTheFilterText(t *testing.T) {
	m := sizedModel(t, 100, 32)
	m.list.SetFilterState(list.Filtering)

	before := m.listView()
	m.list.FilterInput.SetValue("zz")
	after := m.listView()

	if before == after {
		t.Fatal("the cached list was reused after the filter text changed")
	}
	if !strings.Contains(after, "zz") {
		t.Fatal("the rebuilt list does not show the filter text")
	}
}

func TestTheFilterShowsEachLetterAsItIsTyped(t *testing.T) {
	m := sizedModel(t, 100, 32)

	m = feed(t, m, tea.KeyPressMsg{Code: '/', Text: "/"})
	m.render()

	typed := ""
	for _, r := range "chro" {
		typed += string(r)
		m = feed(t, m, tea.KeyPressMsg{Code: r, Text: string(r)})

		if frame := ansi.Strip(m.render()); !strings.Contains(frame, typed) {
			t.Fatalf("after typing %q the frame shows no such text:\n%s", typed, frame)
		}
	}
}

func TestTheFilterNarrowsTheRows(t *testing.T) {
	m := sizedModel(t, 100, 32)

	m = feed(t, m, tea.KeyPressMsg{Code: '/', Text: "/"})
	for _, r := range "chro" {
		m = feed(t, m, tea.KeyPressMsg{Code: r, Text: string(r)})
	}

	if got := len(m.list.VisibleItems()); got != 1 {
		t.Fatalf("visible rows = %d, want 1", got)
	}

	if frame := ansi.Strip(m.render()); strings.Contains(frame, "Brave") {
		t.Fatalf("a filtered-out row is still drawn:\n%s", frame)
	}
}

func TestBackspaceRemovesTheLastLetter(t *testing.T) {
	m := sizedModel(t, 100, 32)

	m = feed(t, m, tea.KeyPressMsg{Code: '/', Text: "/"})
	for _, r := range "chr" {
		m = feed(t, m, tea.KeyPressMsg{Code: r, Text: string(r)})
	}

	m = feed(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace})

	frame := ansi.Strip(m.render())
	if strings.Contains(frame, "Filter: chr") {
		t.Fatalf("the deleted letter is still in the filter box:\n%s", frame)
	}
	if !strings.Contains(frame, "Filter: ch") {
		t.Fatalf("the remaining text is gone as well:\n%s", frame)
	}
}
