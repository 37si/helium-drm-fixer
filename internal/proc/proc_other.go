//go:build !linux && !darwin && !windows

package proc

import "time"

func scan() Snapshot { return Snapshot{At: time.Now()} }
