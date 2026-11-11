package term

import (
	"errors"
	"os"
)

const LaunchedEnv = "HELIUM_DRM_LAUNCHED"

var ErrUnavailable = errors.New("cannot open a terminal window")

type Host string

const (
	HostNone      Host = ""
	HostVSCode    Host = "VS Code"
	HostCursor    Host = "Cursor"
	HostWindsurf  Host = "Windsurf"
	HostZed       Host = "Zed"
	HostJetBrains Host = "JetBrains"
	HostEmacs     Host = "Emacs"
)

type Env struct {
	Host     Host
	Terminal string
}

func (e Env) Embedded() bool { return e.Host != HostNone }

func Detect() Env { return detect(os.Getenv) }

func detect(get func(string) string) Env {
	env := Env{Terminal: firstOf(get, "TERM_PROGRAM", "TERM")}

	switch {
	case get("CURSOR_TRACE_ID") != "":
		env.Host = HostCursor
	case get("TERM_PROGRAM") == "vscode", get("VSCODE_IPC_HOOK_CLI") != "":
		env.Host = HostVSCode
	case get("TERM_PROGRAM") == "windsurf", get("WINDSURF_SESSION_ID") != "":
		env.Host = HostWindsurf
	case get("TERM_PROGRAM") == "zed", get("ZED_TERM") != "":
		env.Host = HostZed
	case get("TERMINAL_EMULATOR") == "JetBrains-JediTerm", get("JETBRAINS_IDE") != "":
		env.Host = HostJetBrains
	case get("INSIDE_EMACS") != "":
		env.Host = HostEmacs
	}

	return env
}

func firstOf(get func(string) string, keys ...string) string {
	for _, key := range keys {
		if value := get(key); value != "" {
			return value
		}
	}

	return ""
}
