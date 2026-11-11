package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"hdf/internal/browser"
	"hdf/internal/fixer"
)

func clickAt(t *testing.T, m *Model, name string) *Model {
	t.Helper()

	m.render()

	for _, h := range m.hits {
		if h.name != name {
			continue
		}

		model, _ := m.Update(tea.MouseClickMsg{
			X:      h.x + h.w/2,
			Y:      h.y,
			Button: tea.MouseLeft,
		})

		return model.(*Model)
	}

	t.Fatalf("no hit region named %q", name)

	return m
}

func pickerModel(t *testing.T) *Model {
	t.Helper()

	m := testModel(t, Options{Fixer: fixer.Options{}})
	model, _ := m.Update(scanDoneMsg{detections: []browser.Detection{
		chromeDetection(true),
		braveDetection(),
	}})

	return model.(*Model)
}

func TestClickingARowSelectsIt(t *testing.T) {
	m := pickerModel(t)

	target := 0
	if m.list.Index() == 0 {
		target = 2
	}

	m = clickAt(t, m, rowName(target))
	if m.list.Index() != target {
		t.Fatalf("index = %d after clicking row %d", m.list.Index(), target)
	}
}

func TestClickingTheSelectedRowContinues(t *testing.T) {
	m := pickerModel(t)

	m.render()

	want := rowName(m.list.Index())
	for _, h := range m.hits {
		if h.name != want {
			continue
		}

		_, cmd := m.Update(tea.MouseClickMsg{X: h.x + 1, Y: h.y, Button: tea.MouseLeft})
		if cmd == nil {
			t.Fatal("clicking the selected row should resolve the plan")
		}

		return
	}

	t.Fatalf("row %q has no hit region", want)
}

func TestClickingAHintRunsTheKey(t *testing.T) {
	m := pickerModel(t)

	m = clickAt(t, m, "hint:doctor")
	if m.screen != screenDoctor {
		t.Fatalf("screen = %v after clicking the doctor hint", m.screen)
	}
}

func TestHoverMarksTheRegionUnderThePointer(t *testing.T) {
	m := pickerModel(t)
	m.render()

	var target hit
	for _, h := range m.hits {
		if h.name == "hint:status" {
			target = h
		}
	}
	if target.w == 0 {
		t.Fatal("no status hint")
	}

	model, _ := m.Update(tea.MouseMotionMsg{X: target.x, Y: target.y})
	m = model.(*Model)

	if m.hover != "hint:status" {
		t.Fatalf("hover = %q", m.hover)
	}

	hot := m.render()

	model, _ = m.Update(tea.MouseMotionMsg{X: 0, Y: 0})
	m = model.(*Model)

	if m.hover != "" {
		t.Fatalf("hover = %q after moving away", m.hover)
	}
	if m.render() == hot {
		t.Fatal("a hovered hint should render differently")
	}
}

func TestWheelMovesTheCursor(t *testing.T) {
	m := pickerModel(t)
	m.list.Select(0)

	model, _ := m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	m = model.(*Model)

	if m.list.Index() != 1 {
		t.Fatalf("index = %d after one wheel notch down", m.list.Index())
	}

	model, _ = m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	m = model.(*Model)

	if m.list.Index() != 0 {
		t.Fatalf("index = %d after one wheel notch up", m.list.Index())
	}
}

func TestBlurDimsAndParksTheClock(t *testing.T) {
	m := pickerModel(t)

	before := m.render()

	model, cmd := m.Update(tea.BlurMsg{})
	m = model.(*Model)

	if m.focused {
		t.Fatal("the model should be unfocused")
	}
	if cmd != nil {
		t.Fatal("a blurred model should stop scheduling ticks")
	}
	if m.animate() != nil {
		t.Fatal("animate should be idle while blurred")
	}
	if !strings.Contains(ansi.Strip(m.render()), "paused") {
		t.Fatal("the header should say the interface is paused")
	}
	if m.render() == before {
		t.Fatal("the palette should be dimmed")
	}

	model, cmd = m.Update(tea.FocusMsg{})
	m = model.(*Model)

	if !m.focused || cmd == nil {
		t.Fatal("focus should restart the clock")
	}
}

func TestCtrlZSuspends(t *testing.T) {
	m := pickerModel(t)

	model, cmd := m.Update(tea.KeyPressMsg{Code: 'z', Mod: tea.ModCtrl})
	m = model.(*Model)

	if cmd == nil {
		t.Fatal("ctrl+z should return a command")
	}
	if msg := cmd(); msg != (tea.SuspendMsg{}) {
		t.Fatalf("ctrl+z produced %#v", msg)
	}
	if m.screen != screenPicking {
		t.Fatalf("screen = %v after suspending", m.screen)
	}
}

func TestTitleAndProgressFollowTheScreen(t *testing.T) {
	m := pickerModel(t)

	if got := m.title(); !strings.Contains(got, "choose a source") {
		t.Fatalf("title = %q", got)
	}
	if m.progressBar() != nil {
		t.Fatal("the picker has no progress to report")
	}

	m.screen = screenApplying
	m.spring.pos = 0.42

	if got := m.title(); !strings.Contains(got, "42%") {
		t.Fatalf("title = %q", got)
	}

	bar := m.progressBar()
	if bar == nil || bar.State != tea.ProgressBarDefault || bar.Value != 42 {
		t.Fatalf("progress bar = %#v", bar)
	}

	status := m.programStatus()
	if status == nil || status.State != tea.ProgramStateWorking || status.Progress != 42 {
		t.Fatalf("program status = %#v", status)
	}

	m.screen = screenReview
	if got := m.programStatus(); got.State != tea.ProgramStateBlocked {
		t.Fatalf("review should report blocked, got %v", got.State)
	}
}

func TestHelpScrollsWithTheKeys(t *testing.T) {
	m := pickerModel(t)

	model, _ := m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	m = model.(*Model)

	if m.screen != screenHelp {
		t.Fatalf("screen = %v after ?", m.screen)
	}

	before := m.viewport.YOffset()

	model, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = model.(*Model)

	if m.viewport.YOffset() <= before {
		t.Fatal("the down arrow should scroll the help text")
	}
}
