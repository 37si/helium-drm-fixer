package browser

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"hdf/internal/cdm"
	"hdf/internal/pathutil"
)

type ID string

const (
	Chrome            ID = "chrome"
	ChromeBeta        ID = "chrome-beta"
	ChromeDev         ID = "chrome-dev"
	ChromeCanary      ID = "chrome-canary"
	Edge              ID = "edge"
	EdgeBeta          ID = "edge-beta"
	EdgeDev           ID = "edge-dev"
	EdgeCanary        ID = "edge-canary"
	Brave             ID = "brave"
	Vivaldi           ID = "vivaldi"
	Opera             ID = "opera"
	OperaGX           ID = "opera-gx"
	Chromium          ID = "chromium"
	UngoogledChromium ID = "ungoogled-chromium"
	Fresh             ID = "fresh"
)

type Browser struct {
	ID       ID
	Name     string
	Priority int
	Aliases  []string
}

var All = []Browser{
	{ID: Chrome, Name: "Google Chrome", Priority: 10, Aliases: []string{"google-chrome", "googlechrome", "chrome-stable"}},
	{ID: ChromeBeta, Name: "Google Chrome Beta", Priority: 12, Aliases: []string{"google-chrome-beta"}},
	{ID: ChromeDev, Name: "Google Chrome Dev", Priority: 14, Aliases: []string{"google-chrome-dev", "chrome-unstable"}},
	{ID: ChromeCanary, Name: "Google Chrome Canary", Priority: 16, Aliases: []string{"chrome-sxs"}},
	{ID: Edge, Name: "Microsoft Edge", Priority: 20, Aliases: []string{"microsoft-edge", "msedge", "edge-stable"}},
	{ID: EdgeBeta, Name: "Microsoft Edge Beta", Priority: 22, Aliases: []string{"microsoft-edge-beta"}},
	{ID: EdgeDev, Name: "Microsoft Edge Dev", Priority: 24, Aliases: []string{"microsoft-edge-dev"}},
	{ID: EdgeCanary, Name: "Microsoft Edge Canary", Priority: 26, Aliases: []string{"microsoft-edge-canary"}},
	{ID: Brave, Name: "Brave", Priority: 30, Aliases: []string{"brave-browser", "bravebrowser"}},
	{ID: Vivaldi, Name: "Vivaldi", Priority: 40, Aliases: []string{"vivaldi-stable"}},
	{ID: Opera, Name: "Opera", Priority: 50, Aliases: []string{"opera-stable"}},
	{ID: OperaGX, Name: "Opera GX", Priority: 52, Aliases: []string{"operagx", "opera gx", "gx"}},
	{ID: Chromium, Name: "Chromium", Priority: 60, Aliases: []string{"chromium-browser"}},
	{ID: UngoogledChromium, Name: "Ungoogled Chromium", Priority: 70, Aliases: []string{"ungoogled", "ungoogled_chromium", "ungoogled chromium"}},
}

type Origin string

const (
	OriginBundled          Origin = "bundled"
	OriginComponentUpdater Origin = "component-updater"
)

type Location struct {
	Path    string
	Origin  Origin
	Version string
}

type Detection struct {
	Browser
	Executable string
	CDM        *Location
}

func (d Detection) Available() bool { return d.CDM != nil }

func Get(id ID) (Browser, bool) {
	for _, b := range All {
		if b.ID == id {
			return b, true
		}
	}
	return Browser{}, false
}

func Parse(name string) (ID, bool) {
	normalized := strings.ToLower(strings.TrimSpace(name))
	squashed := squash(normalized)

	if squashed == string(Fresh) || squashed == "download" || squashed == "mozilla" {
		return Fresh, true
	}

	for _, b := range All {
		if string(b.ID) == normalized {
			return b.ID, true
		}
		for _, alias := range b.Aliases {
			if alias == normalized {
				return b.ID, true
			}
		}
	}

	for _, b := range All {
		if squash(string(b.ID)) == squashed {
			return b.ID, true
		}
		for _, alias := range b.Aliases {
			if squash(alias) == squashed {
				return b.ID, true
			}
		}
	}

	return "", false
}

func squash(s string) string {
	return strings.NewReplacer(" ", "", "-", "", "_", "").Replace(s)
}

func Detect(ctx context.Context) []Detection {
	detections := make([]Detection, 0, len(All))

	DetectStream(ctx, func(detection Detection) {
		if detection.Executable != "" || detection.CDM != nil {
			detections = append(detections, detection)
		}
	})

	sort.Slice(detections, func(i, j int) bool {
		return detections[i].Priority < detections[j].Priority
	})

	return detections
}

func DetectStream(ctx context.Context, yield func(Detection)) {
	results := make(chan Detection, len(All))

	var wg sync.WaitGroup
	for _, b := range All {
		wg.Add(1)
		go func(b Browser) {
			defer wg.Done()
			results <- probe(ctx, b)
		}(b)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for detection := range results {
		yield(detection)
	}
}

func DetectOne(ctx context.Context, id ID) (Detection, bool) {
	b, ok := Get(id)
	if !ok {
		return Detection{}, false
	}

	results := detect(ctx, []Browser{b})
	if len(results) == 0 {
		return Detection{}, false
	}
	return results[0], true
}

func detect(ctx context.Context, browsers []Browser) []Detection {
	out := make([]Detection, len(browsers))

	var wg sync.WaitGroup
	for i, b := range browsers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out[i] = probe(ctx, b)
		}()
	}
	wg.Wait()

	detections := make([]Detection, 0, len(out))
	for _, d := range out {
		if d.Executable != "" || d.CDM != nil {
			detections = append(detections, d)
		}
	}

	sort.Slice(detections, func(i, j int) bool {
		return detections[i].Priority < detections[j].Priority
	})

	return detections
}

func probe(ctx context.Context, b Browser) Detection {
	detection := Detection{Browser: b}

	paths, ok := platformPaths[b.ID]
	if !ok {
		return detection
	}

	detection.Executable = findExecutable(paths)

	if location, ok := discover(paths, detection.Executable); ok {
		detection.CDM = &location
	}

	if err := ctx.Err(); err != nil {
		return Detection{Browser: b}
	}

	return detection
}

type candidate struct {
	path   string
	origin Origin
}

type paths struct {
	executables  []string
	installRoots []string
	userDataDirs []string
	appBundles   []string
}

func discover(paths paths, executable string) (Location, bool) {
	roots := installRoots(paths, executable)

	var candidates []candidate
	for _, root := range roots {
		for _, name := range cdm.DirNames() {
			candidates = append(candidates, candidate{filepath.Join(root, name), OriginBundled})
		}
		for _, version := range pathutil.VersionDirs(root) {
			for _, name := range cdm.DirNames() {
				candidates = append(candidates, candidate{filepath.Join(root, version, name), OriginBundled})
			}
		}
	}

	for _, path := range frameworkCandidates(paths) {
		candidates = append(candidates, candidate{path, OriginBundled})
	}

	for _, dir := range userDataDirs(paths) {
		root := filepath.Join(dir, "WidevineCdm")
		candidates = append(candidates, candidate{root, OriginComponentUpdater})
		for _, version := range pathutil.VersionDirs(root) {
			candidates = append(candidates, candidate{filepath.Join(root, version), OriginComponentUpdater})
		}
	}

	seen := make(map[string]struct{}, len(candidates))
	for _, c := range candidates {
		if _, ok := seen[c.path]; ok {
			continue
		}
		seen[c.path] = struct{}{}

		if !cdm.Valid(c.path) {
			continue
		}

		location := Location{Path: c.path, Origin: c.origin}
		if version, err := cdm.Version(c.path); err == nil {
			location.Version = version
		}
		return location, true
	}

	return Location{}, false
}

func Locate(path string) (Location, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return Location{}, false
	}

	resolved := path
	if real, err := filepath.EvalSymlinks(path); err == nil {
		resolved = real
	}

	if location, ok := locationAt(resolved); ok {
		return location, true
	}

	if !info.IsDir() {
		dir := filepath.Dir(resolved)

		return discover(paths{
			installRoots: []string{dir, filepath.Dir(dir)},
			appBundles:   appBundlesAround(dir),
		}, resolved)
	}

	roots := []string{resolved}
	bundles := appBundlesAround(resolved)

	var best Location
	bestFound := false

	if entries, err := os.ReadDir(resolved); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			child := filepath.Join(resolved, entry.Name())
			if location, ok := locationAt(child); ok {
				if !bestFound || pathutil.CompareVersions(location.Version, best.Version) > 0 {
					best = location
					bestFound = true
				}
			}
			if strings.EqualFold(entry.Name(), "Application") {
				roots = append(roots, child)
			}
			if strings.EqualFold(filepath.Ext(entry.Name()), ".app") {
				roots = append(roots, child)
			}
		}
	}

	if bestFound {
		return best, true
	}

	return discover(paths{installRoots: roots, appBundles: bundles}, "")
}

func locationAt(path string) (Location, bool) {
	if !cdm.Valid(path) {
		return Location{}, false
	}

	location := Location{Path: path, Origin: OriginBundled}
	if version, err := cdm.Version(path); err == nil {
		location.Version = version
	}

	return location, true
}

func appBundlesAround(path string) []string {
	for probe := path; ; probe = filepath.Dir(probe) {
		if strings.EqualFold(filepath.Ext(probe), ".app") {
			return []string{probe}
		}

		parent := filepath.Dir(probe)
		if parent == probe {
			return nil
		}
	}
}

func installRoots(paths paths, executable string) []string {
	var roots []string

	for _, root := range pathutil.ExpandAll(paths.installRoots) {
		roots = append(roots, root)
	}

	if executable != "" {
		dir := filepath.Dir(executable)
		roots = append(roots, dir, filepath.Dir(dir))
	}

	roots = append(roots, pathutil.ExpandAll(paths.appBundles)...)

	return unique(roots)
}

func userDataDirs(paths paths) []string {
	return pathutil.ExpandAll(paths.userDataDirs)
}

func frameworkCandidates(paths paths) []string {
	var out []string

	for _, bundle := range pathutil.ExpandAll(paths.appBundles) {
		frameworks := filepath.Join(bundle, "Contents", "Frameworks")
		entries, err := os.ReadDir(frameworks)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() || !strings.HasSuffix(entry.Name(), ".framework") {
				continue
			}

			versionsRoot := filepath.Join(frameworks, entry.Name(), "Versions")
			for _, version := range pathutil.VersionDirs(versionsRoot) {
				for _, name := range cdm.DirNames() {
					out = append(out,
						filepath.Join(versionsRoot, version, "Libraries", name),
						filepath.Join(versionsRoot, version, name),
					)
				}
			}
		}
	}

	return out
}

func findExecutable(paths paths) string {
	candidates := pathutil.ExpandAll(paths.executables)

	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		if resolved, err := filepath.EvalSymlinks(candidate); err == nil {
			return resolved
		}
		return candidate
	}

	for _, candidate := range candidates {
		if !onPath(candidate) {
			continue
		}

		name := strings.TrimSuffix(filepath.Base(candidate), ".exe")
		if found, err := exec.LookPath(name); err == nil {
			if resolved, err := filepath.EvalSymlinks(found); err == nil {
				return resolved
			}
			return found
		}
	}

	return ""
}

var binDirs = []string{"/usr/bin", "/usr/local/bin", "/bin", "/sbin", "/snap/bin", "/opt/homebrew/bin"}

func onPath(candidate string) bool {
	dir := filepath.Dir(candidate)

	for _, bin := range binDirs {
		if sameDir(dir, filepath.FromSlash(bin)) {
			return true
		}
	}

	for _, entry := range filepath.SplitList(os.Getenv("PATH")) {
		if entry == "" {
			continue
		}
		if sameDir(dir, entry) {
			return true
		}
	}

	return false
}

func sameDir(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}

	return a == b
}

func unique(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))

	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}

	return out
}
