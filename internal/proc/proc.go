package proc

import (
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Process struct {
	PID    int
	Parent int
	Name   string
	Exe    string
	Args   []string
}

type Snapshot struct {
	At    time.Time
	Procs []Process
}

type Status struct {
	Running bool
	Browser bool
	Matches bool
	PIDs    []int
	Paths   []string
}

func (s Status) Names() []string {
	seen := make(map[string]struct{}, len(s.Paths))
	out := make([]string, 0, len(s.Paths))

	for _, path := range s.Paths {
		name := filepath.Base(path)
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}

	sort.Strings(out)

	return out
}

func (s Status) Describe() string {
	switch {
	case !s.Running:
		return ""
	case len(s.Paths) == 1:
		return s.Paths[0]
	}

	return strings.Join(s.Paths, ", ")
}

func now() time.Time { return time.Now() }

const TTL = 1500 * time.Millisecond

var (
	mu       sync.Mutex
	last     Snapshot
	lastRead time.Time
)

func Current(age time.Duration) Snapshot {
	mu.Lock()
	defer mu.Unlock()

	if age > 0 && !lastRead.IsZero() && time.Since(lastRead) < age {
		return last
	}

	last = scan()
	lastRead = time.Now()

	return last
}

func Invalidate() {
	mu.Lock()
	lastRead = time.Time{}
	mu.Unlock()
}

func Running(names ...string) []string {
	found := Current(TTL).Find(names...)

	seen := make(map[string]struct{}, len(found))
	out := make([]string, 0, len(found))

	for _, p := range found {
		name := normalize(p.Name)
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}

	return out
}

func RunningExe(path string) bool {
	if path == "" {
		return false
	}

	return Current(TTL).RunningExe(path)
}

func (s Snapshot) Find(names ...string) []Process {
	if len(names) == 0 {
		return nil
	}

	out := make([]Process, 0, 4)

	for _, p := range s.Procs {
		if matchesAny(p, names) {
			out = append(out, p)
		}
	}

	return out
}

func (s Snapshot) Has(names ...string) bool {
	for _, p := range s.Procs {
		if matchesAny(p, names) {
			return true
		}
	}

	return false
}

func (s Snapshot) RunningExe(path string) bool {
	want := foldPath(canonical(path))
	if want == "" {
		return false
	}

	name := normalize(filepath.Base(want))

	for _, p := range s.Procs {
		if normalize(p.Name) != name {
			continue
		}
		if p.Exe == "" || foldPath(canonical(p.Exe)) == want {
			return true
		}
	}

	for _, p := range s.Procs {
		if p.Exe != "" && foldPath(canonical(p.Exe)) == want {
			return true
		}
	}

	return false
}

func (s Snapshot) Helium(root string) Status {
	var status Status

	wantRoot := canonical(root)
	seenPID := make(map[int]struct{}, 4)
	seenPath := make(map[string]struct{}, 4)
	matched := make([]Process, 0, 8)

	for _, p := range s.Procs {
		if !isHelium(p, wantRoot) {
			continue
		}
		if _, ok := seenPID[p.PID]; ok {
			continue
		}
		seenPID[p.PID] = struct{}{}
		matched = append(matched, p)
	}

	for _, p := range matched {
		status.Running = true
		status.PIDs = append(status.PIDs, p.PID)

		if !isHelper(p) && !hasHeliumParent(p, seenPID) {
			status.Browser = true
		}

		path := p.Exe
		if path == "" && len(p.Args) > 0 {
			path = p.Args[0]
		}
		if path == "" {
			continue
		}

		if _, ok := seenPath[path]; !ok {
			seenPath[path] = struct{}{}
			status.Paths = append(status.Paths, path)
		}

		if wantRoot != "" && underRoot(path, wantRoot) {
			status.Matches = true
		}
	}

	return status
}

func hasHeliumParent(p Process, pids map[int]struct{}) bool {
	if len(p.Args) > 0 || p.Parent == 0 {
		return false
	}

	_, ok := pids[p.Parent]

	return ok
}

func isHelium(p Process, root string) bool {
	if looksHelium(p.Name) || looksHelium(base(p.Exe)) {
		return true
	}

	if len(p.Args) > 0 && looksHelium(base(p.Args[0])) {
		return true
	}

	if !chromiumName(p.Name) && !chromiumName(base(p.Exe)) {
		return false
	}

	if root != "" && underRoot(p.Exe, root) {
		return true
	}

	return heliumPath(p.Exe)
}

func looksHelium(name string) bool {
	name = normalize(name)
	if name == "" {
		return false
	}

	return name == "helium" || strings.HasPrefix(name, "helium-") || strings.HasPrefix(name, "helium_")
}

func isHelper(p Process) bool {
	for _, arg := range p.Args {
		if strings.HasPrefix(arg, "--type=") {
			return true
		}
	}

	return false
}

func underRoot(path, root string) bool {
	path = canonical(path)
	if path == "" {
		return false
	}

	p := foldPath(path)
	r := strings.TrimRight(foldPath(root), `/\`)

	if p == r {
		return true
	}

	for _, sep := range []string{"/", `\`} {
		if strings.HasPrefix(p, r+sep) {
			return true
		}
	}

	if start := strings.Index(p, "/.mount_"); start >= 0 {
		rest := p[start+len("/.mount_"):]
		if slash := strings.Index(rest, "/"); slash >= 0 {
			inner := strings.Trim(rest[slash+1:], `/\`)
			root := strings.Trim(r, `/\`)
			if root != "" && (inner == root || strings.HasPrefix(inner, root+"/")) {
				return true
			}
		}
	}

	return false
}

func matchesAny(p Process, names []string) bool {
	if matches(p.Name, names) {
		return true
	}

	if p.Exe != "" && matches(filepath.Base(p.Exe), names) {
		return true
	}

	if len(p.Args) > 0 && matches(filepath.Base(p.Args[0]), names) {
		return true
	}

	return false
}
