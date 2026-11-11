//go:build linux

package term

import (
	"errors"
	"strings"
	"testing"
)

func TestEmulatorArgsCarryTheScript(t *testing.T) {
	for _, em := range emulators {
		args := em.args("/tmp/run.sh")
		if len(args) == 0 {
			t.Fatalf("%s has no arguments", em.bin)
		}
		if !strings.Contains(strings.Join(args, " "), "/tmp/run.sh") {
			t.Fatalf("%s drops the script: %v", em.bin, args)
		}
	}
}

func TestOpenUnknownTerminal(t *testing.T) {
	err := Open([]string{"true"}, Options{Terminal: "no-such-terminal-binary"})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
}
