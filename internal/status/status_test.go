package status

import (
	"context"
	"testing"

	"hdf/internal/browser"
)

func detection(id browser.ID, name, version string) browser.Detection {
	found := browser.Detection{Browser: browser.Browser{ID: id, Name: name}, Executable: "/usr/bin/" + string(id)}
	if version != "" {
		found.CDM = &browser.Location{Path: "/opt/" + string(id) + "/WidevineCdm", Version: version}
	}

	return found
}

func TestNewestPicksHighestVersion(t *testing.T) {
	got := newest(context.Background(), []browser.Detection{
		detection(browser.Chrome, "Google Chrome", "4.10.2900.0"),
		detection(browser.Brave, "Brave", "4.10.3112.0"),
		detection(browser.Vivaldi, "Vivaldi", ""),
	})

	if !got.Available || got.Name != "Brave" || got.Version != "4.10.3112.0" {
		t.Fatalf("newest = %+v", got)
	}
}

func TestNewestWithoutSources(t *testing.T) {
	got := newest(context.Background(), []browser.Detection{detection(browser.Edge, "Microsoft Edge", "")})
	if got.Available {
		t.Fatalf("newest = %+v", got)
	}
}

func TestOutdated(t *testing.T) {
	cases := []struct {
		name     string
		cdm      *Installed
		newest   Candidate
		outdated bool
	}{
		{"older installed", &Installed{Version: "4.10.2900.0"}, Candidate{Available: true, Version: "4.10.3112.0"}, true},
		{"same version", &Installed{Version: "4.10.3112.0"}, Candidate{Available: true, Version: "4.10.3112.0"}, false},
		{"newer installed", &Installed{Version: "4.10.3200.0"}, Candidate{Available: true, Version: "4.10.3112.0"}, false},
		{"nothing installed", nil, Candidate{Available: true, Version: "4.10.3112.0"}, false},
		{"no source", &Installed{Version: "4.10.2900.0"}, Candidate{}, false},
	}

	for _, c := range cases {
		report := Report{CDM: c.cdm, Newest: c.newest}
		if got := report.Outdated(); got != c.outdated {
			t.Fatalf("%s: Outdated = %v", c.name, got)
		}
	}
}
