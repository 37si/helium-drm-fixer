package term

import "testing"

func envOf(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestDetectEmbedded(t *testing.T) {
	cases := []struct {
		name   string
		values map[string]string
		want   Host
	}{
		{"vscode", map[string]string{"TERM_PROGRAM": "vscode"}, HostVSCode},
		{"vscode ipc hook", map[string]string{"VSCODE_IPC_HOOK_CLI": "/tmp/x"}, HostVSCode},
		{"cursor wins over vscode", map[string]string{"TERM_PROGRAM": "vscode", "CURSOR_TRACE_ID": "abc"}, HostCursor},
		{"windsurf", map[string]string{"TERM_PROGRAM": "windsurf"}, HostWindsurf},
		{"zed", map[string]string{"ZED_TERM": "true"}, HostZed},
		{"jetbrains", map[string]string{"TERMINAL_EMULATOR": "JetBrains-JediTerm"}, HostJetBrains},
		{"emacs", map[string]string{"INSIDE_EMACS": "29.1"}, HostEmacs},
		{"iterm is a real terminal", map[string]string{"TERM_PROGRAM": "iTerm.app"}, HostNone},
		{"apple terminal", map[string]string{"TERM_PROGRAM": "Apple_Terminal"}, HostNone},
		{"windows terminal", map[string]string{"WT_SESSION": "guid"}, HostNone},
		{"bare tty", map[string]string{"TERM": "xterm-256color"}, HostNone},
		{"empty", map[string]string{}, HostNone},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := detect(envOf(c.values))
			if got.Host != c.want {
				t.Fatalf("host = %q, want %q", got.Host, c.want)
			}
			if got.Embedded() != (c.want != HostNone) {
				t.Fatalf("embedded = %v for %q", got.Embedded(), got.Host)
			}
		})
	}
}

func TestDetectTerminalName(t *testing.T) {
	if got := detect(envOf(map[string]string{"TERM_PROGRAM": "ghostty"})); got.Terminal != "ghostty" {
		t.Fatalf("terminal = %q", got.Terminal)
	}
	if got := detect(envOf(map[string]string{"TERM": "screen"})); got.Terminal != "screen" {
		t.Fatalf("terminal = %q", got.Terminal)
	}
}
