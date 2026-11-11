package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"hdf/internal/backup"
)

type usageRow struct {
	names string
	arg   string
	help  string
}

type usageGroup struct {
	title string
	rows  []usageRow
}

var usageGroups = []usageGroup{
	{
		title: "source",
		rows: []usageRow{
			{"-b, --browser", "NAME", "only use this browser as the source"},
			{"    --source-path", "PATH", "WidevineCdm directory, install directory or browser binary"},
			{"    --newest", "", "prefer the highest CDM version over browser order"},
			{"    --fresh", "", "download the newest WidevineCdm from Mozilla"},
			{"    --gmp-version", "VERSION", "Firefox version whose update endpoint to ask"},
		},
	},
	{
		title: "target",
		rows: []usageRow{
			{"    --helium", "DIR", "Helium install directory"},
			{"    --dry-run, -n", "", "report without writing anything"},
		},
	},
	{
		title: "keeping copies",
		rows: []usageRow{
			{"    --keep", "N", "how many backups to keep, 0 disables them"},
			{"    --no-backup", "", "do not keep a copy of the directory being replaced"},
			{"    --rollback", "ID", "restore a backup: an id, or \"latest\""},
			{"    --backups", "", "list saved copies"},
			{"    --clear-backups", "", "delete every saved backup (needs --yes or two prompts)"},
		},
	},
	{
		title: "inspecting",
		rows: []usageRow{
			{"-s, --status", "", "show what Helium has and whether a newer CDM exists"},
			{"-d, --doctor", "", "run health checks and explain what to fix"},
			{"    --list, -l", "", "list detected browsers and exit"},
			{"    --check", "", "resolve source and target, then exit"},
			{"    --history", "", "list previous installs"},
			{"    --verify", "", "open a Widevine test stream in Helium"},
		},
	},
	{
		title: "staying current",
		rows: []usageRow{
			{"    --watch", "", "reinstall automatically when Helium updates"},
			{"    --interval", "DURATION", "poll interval for --watch"},
			{"    --upgrade", "", "install the newest hdf release"},
			{"    --repo", "OWNER/NAME", "GitHub repository for update checks"},
			{"    --online", "", "let --doctor check for a newer release"},
		},
	},
	{
		title: "output",
		rows: []usageRow{
			{"    --json", "", "machine readable output"},
			{"-q, --quiet", "", "no progress output"},
			{"    --log-file", "PATH", "write logs to this file"},
			{"-v, --verbose", "", "log every lookup"},
		},
	},
	{
		title: "how it runs",
		rows: []usageRow{
			{"-y, --yes", "", "install without the interface"},
			{"    --no-anim", "", "turn off interface animations"},
			{"    --no-mouse", "", "ignore the mouse"},
			{"    --external", "", "always open the interface in its own window"},
			{"    --no-external", "", "never leave the current terminal"},
			{"    --terminal", "NAME", "terminal emulator to open"},
		},
	},
	{
		title: "settings",
		rows: []usageRow{
			{"    --config", "PATH", "settings file"},
			{"    --print-config", "", "show the settings in effect and exit"},
			{"    --completion", "SHELL", "print a completion script"},
			{"    --version", "", "print the version and exit"},
		},
	},
}

func usageText() string {
	var b strings.Builder

	fmt.Fprintf(&b, "hdf %s\n\n", version)
	b.WriteString("installs WidevineCdm from a Chromium-based browser into Helium\n\n")

	b.WriteString("usage\n")
	for _, line := range []string{
		"hdf              open the interface",
		"hdf --yes        install without the interface",
		"hdf --status     show what Helium has",
		"hdf --watch      keep it installed",
	} {
		fmt.Fprintf(&b, "  %s\n", line)
	}

	width := 0
	for _, group := range usageGroups {
		for _, row := range group.rows {
			width = max(width, len(row.names+" "+row.arg))
		}
	}

	for _, group := range usageGroups {
		fmt.Fprintf(&b, "\n%s\n", group.title)

		for _, row := range group.rows {
			name := strings.TrimRight(row.names+" "+row.arg, " ")
			fmt.Fprintf(&b, "  %-*s  %s\n", width, name, row.help)
		}
	}

	b.WriteString("\n--keep defaults to ")
	b.WriteString(fmt.Sprint(backup.DefaultKeep))
	b.WriteString(", --interval to ")
	b.WriteString((30 * time.Second).String())
	b.WriteString(".\n")

	return b.String()
}

func printConfig(parsed invocation) error {
	state := "missing"
	if _, err := os.Stat(parsed.file); err == nil {
		state = "present"
	}

	fmt.Printf("settings  %s (%s)\n\n", parsed.file, state)

	rows := [][3]string{
		{"browser", orAuto(parsed.cfg.browser), parsed.origin("browser", "b", parsed.settings.Browser != "")},
		{"helium", orAuto(parsed.cfg.helium), parsed.origin("helium", "", parsed.settings.Helium != "")},
		{"repo", orAuto(repository(parsed.cfg)), parsed.origin("repo", "", parsed.settings.Repo != "")},
		{"keep", fmt.Sprint(parsed.cfg.keep), parsed.origin("keep", "", parsed.settings.Keep != nil)},
		{"interval", parsed.cfg.interval.String(), parsed.origin("interval", "", parsed.settings.Interval != "")},
		{"backup", yesNo(!parsed.cfg.noBackup && parsed.cfg.keep > 0), parsed.origin("no-backup", "keep", parsed.settings.NoBackup != nil || parsed.settings.Keep != nil)},
		{"newest", yesNo(parsed.cfg.newest), parsed.origin("newest", "", parsed.settings.Newest != nil)},
		{"external", yesNo(parsed.cfg.external), parsed.origin("external", "", parsed.settings.External != nil)},
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, row := range rows {
		fmt.Fprintf(w, "%-10s%-32s%s\n", row[0], row[1], row[2])
	}

	if err := w.Flush(); err != nil {
		return err
	}

	fmt.Printf("\nlog       %s\n", parsed.cfg.logPath)

	return nil
}

func (parsed invocation) origin(flagName, alias string, fromFile bool) string {
	switch {
	case parsed.set[flagName] || (alias != "" && parsed.set[alias]):
		return "flag"
	case fromFile:
		return "settings"
	}

	return "default"
}

func orAuto(value string) string {
	if value == "" {
		return "auto"
	}

	return value
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}

	return "no"
}
