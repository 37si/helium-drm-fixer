package tui

import (
	"context"
	"testing"

	"charm.land/bubbles/v2/list"

	"hdf/internal/browser"
	"hdf/internal/fixer"
	"hdf/internal/helium"
	"hdf/internal/proc"
	"hdf/internal/status"
)

func benchModel(b *testing.B, width, height int) *Model {
	b.Helper()

	m := New(context.Background(), Options{Version: "bench"}, nil)
	m.width, m.height = width, height
	m.resize()

	m.detections = []browser.Detection{
		{Browser: browser.All[0], Executable: "/usr/bin/google-chrome", CDM: &browser.Location{Path: "/opt/google/chrome/WidevineCdm", Version: "4.10.2934.0", Origin: browser.OriginBundled}},
		{Browser: browser.All[8], Executable: "/usr/bin/brave-browser"},
	}
	m.hasFresh = true

	items := make([]list.Item, 0, 3)
	items = append(items, sourceItem{fresh: true})
	for _, detection := range m.detections {
		items = append(items, sourceItem{detection: detection})
	}
	m.list.SetItems(items)
	m.list.SetSize(m.layout().text(m.layout().left), m.layout().body-2)

	m.heliumDir = "/opt/helium"
	m.held = "4.10.2891.0"
	m.running = proc.Status{Running: true, Matches: true, PIDs: []int{4242}, Paths: []string{"/opt/helium/helium"}}
	m.plan = fixer.Plan{
		Target:   "/root/.config/net.imput.helium/WidevineCdm/4.10.2934.0",
		HintFile: "/root/.config/net.imput.helium/WidevineCdm/latest-component-updated-widevine-cdm",
		Source:   fixer.Source{Name: "Google Chrome", Version: "4.10.2934.0", Path: "/opt/google/chrome/WidevineCdm"},
		Helium:   helium.Install{Root: "/opt/helium", Dir: "/opt/helium", Exe: "/opt/helium/helium"},
	}
	m.planReady = true
	m.report = status.Report{
		Found: true,
		CDM: &status.Installed{
			Path: "/root/.config/net.imput.helium/WidevineCdm/4.10.2934.0", Version: "4.10.2934.0",
			Managed: true, Source: "Google Chrome", Size: 19 << 20, Files: 7,
		},
		Newest:  status.Candidate{Available: true, Name: "Google Chrome", Version: "4.10.3112.0"},
		Running: m.running,
		Backups: 2,
	}
	m.reportAt = m.started

	return m
}

func BenchmarkRenderAnimatedPicking(b *testing.B) {
	m := benchModel(b, 120, 40)
	m.screen = screenPicking

	b.ReportAllocs()

	for b.Loop() {
		m.frame++
		m.render()
	}
}

func BenchmarkRenderAnimatedPickingStill(b *testing.B) {
	m := benchModel(b, 120, 40)
	m.screen = screenPicking
	m.anim = false

	b.ReportAllocs()

	for b.Loop() {
		m.render()
	}
}

func BenchmarkRenderStatus(b *testing.B) {
	m := benchModel(b, 120, 40)
	m.screen = screenStatus
	m.anim = false

	b.ReportAllocs()

	for b.Loop() {
		m.render()
	}
}
