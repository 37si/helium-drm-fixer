package tui

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"charm.land/log/v2"

	"hdf/internal/browser"
	"hdf/internal/doctor"
	"hdf/internal/fixer"
	"hdf/internal/helium"
	"hdf/internal/proc"
	"hdf/internal/state"
	"hdf/internal/status"
	"hdf/internal/verify"
	"hdf/internal/watch"
)

type screen int

const (
	screenScanning screen = iota
	screenPicking
	screenReview
	screenApplying
	screenDone
	screenFailed
	screenHelp
	screenStatus
	screenDoctor
	screenWatch
	screenClear
)

type stepState int

const (
	stepPending stepState = iota
	stepActive
	stepDone
)

type applyStep struct {
	name  string
	state stepState
}

type Options struct {
	Fixer    fixer.Options
	DryRun   bool
	LogPath  string
	Version  string
	Repo     string
	Online   bool
	NoAnim   bool
	NoMouse  bool
	Interval time.Duration
}

type (
	scanMsg     struct{ detection browser.Detection }
	scanDoneMsg struct {
		detections []browser.Detection

		helium string
		held   string
	}
	planMsg struct {
		plan  fixer.Plan
		err   error
		apply bool
	}
	stepMsg struct {
		step     string
		fraction float64
	}
	applyMsg struct {
		result fixer.Result
		err    error
	}
	tickMsg struct{}
	pollMsg struct{}
)

const (
	animTick  = 80 * time.Millisecond
	animCycle = 48
	pollEvery = 4 * time.Second
)

type springState struct {
	pos float64
	vel float64
}

func (s *springState) step(target float64) bool {
	const (
		stiffness = 90.0
		damping   = 16.0
		dt        = 1.0 / 60.0
	)

	accel := stiffness*(target-s.pos) - damping*s.vel
	s.vel += accel * dt
	s.pos += s.vel * dt

	if s.pos < 0 {
		s.pos, s.vel = 0, 0
	}

	if math.Abs(target-s.pos) < 0.001 && math.Abs(s.vel) < 0.001 {
		s.pos, s.vel = target, 0
		return true
	}

	return false
}

type listKey struct {
	index  int
	hover  string
	width  int
	height int
	items  int
	filter list.FilterState
	typed  string
}

type Model struct {
	ctx            context.Context
	opts           Options
	logger         *log.Logger
	width          int
	height         int
	screen         screen
	spinner        spinner.Model
	list           list.Model
	form           *huh.Form
	progress       progress.Model
	viewport       viewport.Model
	spring         springState
	target         float64
	animating      bool
	frame          int
	anim           bool
	history        []screen
	flash          map[browser.ID]int
	hits           []hit
	hover          string
	focused        bool
	frozen         float64
	caps           capabilities
	seen           map[browser.ID]browser.Detection
	scanned        int
	scanTotal      int
	scanCh         <-chan tea.Msg
	detections     []browser.Detection
	hasFresh       bool
	plan           fixer.Plan
	planReady      bool
	result         fixer.Result
	err            error
	steps          []applyStep
	step           string
	applyC         <-chan tea.Msg
	started        time.Time
	finished       time.Time
	heliumPath     string
	heliumDir      string
	held           string
	dryRun         bool
	report         status.Report
	reportAt       time.Time
	checks         []doctor.Check
	scroll         int
	running        proc.Status
	runningAt      time.Time
	watchEvents    []watch.Event
	watchCh        <-chan tea.Msg
	watchCancel    context.CancelFunc
	watchPaused    bool
	watchInterval  time.Duration
	watchScroll    int
	quitAfterApply bool
	clearCount     int
	clearBytes     int64
	clearDir       string
	clearStage     int
	clearBusy      bool
	verify         verifyState
	stream         verify.Stream
	undone         bool
	notice         string
	noticeAt       time.Time
	last           recentSource
	listCache      string
	listCacheKey   listKey
	helpCache      string
	helpWidth      int
}

type recentSource struct {
	id browser.ID
	at time.Time
}

func lastSource(dir string) recentSource {
	records, err := state.Open(dir).Load()
	if err != nil {
		return recentSource{}
	}

	for i := len(records) - 1; i >= 0; i-- {
		if records[i].Method != state.MethodCopy || records[i].SourceID == "" {
			continue
		}

		return recentSource{id: browser.ID(records[i].SourceID), at: records[i].InstalledAt}
	}

	return recentSource{}
}

func New(ctx context.Context, opts Options, logger *log.Logger) *Model {
	interval := opts.Interval
	if interval == 0 {
		interval = 30 * time.Second
	}

	m := &Model{
		ctx:       ctx,
		opts:      opts,
		logger:    logger,
		screen:    screenScanning,
		dryRun:    opts.DryRun,
		started:   time.Now(),
		seen:      make(map[browser.ID]browser.Detection, len(browser.All)),
		scanTotal: len(browser.All),
		last:      lastSource(opts.Fixer.StateDir),
		anim:      !opts.NoAnim,
		focused:   true,
		flash:     make(map[browser.ID]int, len(browser.All)),

		watchInterval: clampDuration(interval, watch.MinInterval, 10*time.Minute),
	}

	m.spinner = spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(theme.Accent))

	m.progress = progress.New(
		progress.WithWidth(48),
		progress.WithColors(theme.Primary, theme.Secondary, theme.Tertiary),
		progress.WithFillCharacters('█', '░'),
		progress.WithoutPercentage(),
	)

	m.viewport = viewport.New()

	m.list = list.New(nil, newSourceDelegate(&m.hover), 0, 0)
	m.list.SetShowTitle(false)
	m.list.SetShowHelp(false)
	m.list.SetShowStatusBar(false)
	m.list.SetShowPagination(false)
	m.list.DisableQuitKeybindings()

	return m
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.startScan(), m.animate(), tea.RequestBackgroundColor, poll())
}

func poll() tea.Cmd {
	return tea.Tick(pollEvery, func(time.Time) tea.Msg { return pollMsg{} })
}

func (m *Model) animate() tea.Cmd {
	if !m.anim || !m.focused || !m.animates() {
		return nil
	}

	return tea.Tick(animTick, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m *Model) animates() bool {
	switch m.screen {
	case screenScanning, screenPicking, screenReview, screenApplying:
		return true
	}

	return false
}

func (m *Model) setFocused(on bool) tea.Cmd {
	if m.focused == on {
		return nil
	}

	m.focused = on
	SetDimmed(!on)

	m.listCache, m.listCacheKey = "", listKey{}

	if !on {
		m.frozen = math.Mod(float64(m.frame)/animCycle, 1)
		return nil
	}

	return m.animate()
}

type capabilities struct {
	kitty   bool
	release bool
	status  bool
}

func (m *Model) phase() float64 {
	if !m.anim {
		return 0
	}
	if !m.focused {
		return m.frozen
	}
	if !m.animates() {
		return 0
	}

	return math.Mod(float64(m.frame)/animCycle, 1)
}

func (m *Model) startScan() tea.Cmd {
	ch := make(chan tea.Msg, len(browser.All)+1)
	m.scanCh = ch
	ctx := m.ctx

	go func() {
		defer close(ch)

		var found []browser.Detection
		browser.DetectStream(ctx, func(detection browser.Detection) {
			if detection.Executable != "" || detection.CDM != nil {
				found = append(found, detection)
			}
			ch <- scanMsg{detection: detection}
		})

		var dir, held string
		if install, err := helium.Find(m.opts.Fixer.HeliumPath); err == nil {
			dir = install.Dir
			if existing := helium.Installed(install); len(existing) > 0 {
				held = existing[0].Version
			}
		}

		ch <- scanDoneMsg{detections: found, helium: dir, held: held}
	}()

	return waitFor(ch)
}

func planCmd(ctx context.Context, opts fixer.Options, logger *log.Logger, apply bool) tea.Cmd {
	return func() tea.Msg {
		plan, err := fixer.Resolve(ctx, opts, logger, nil)
		return planMsg{plan: plan, err: err, apply: apply}
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		return m, nil

	case tea.BackgroundColorMsg:

		Configure(theme.Profile, msg.IsDark(), msg.Color)
		m.retheme()
		return m, nil

	case tickMsg:
		m.frame++

		var cmd tea.Cmd
		if m.animating {
			if m.spring.step(m.target) {
				m.animating = false
			} else {
				cmd = m.animate()
			}
		}

		if cmd == nil {
			cmd = m.animate()
		}

		return m, cmd

	case pollMsg:
		return m, tea.Batch(m.refreshRunning(), poll())

	case tea.FocusMsg:
		return m, m.setFocused(true)

	case tea.BlurMsg:
		return m, m.setFocused(false)

	case tea.KeyboardEnhancementsMsg:
		m.caps.kitty = msg.SupportsKeyDisambiguation()
		m.caps.release = msg.SupportsEventTypes()
		m.helpCache = ""
		return m, nil

	case tea.ProgramStatusSupportMsg:
		m.caps.status = true
		return m, nil

	case tea.MouseMsg:
		return m, m.onMouse(msg)

	case tea.SuspendMsg:
		return m, nil

	case tea.ResumeMsg:
		return m, m.animate()

	case scanMsg:
		m.seen[msg.detection.ID] = msg.detection
		m.flash[msg.detection.ID] = m.frame
		m.scanned++
		return m, waitFor(m.scanCh)

	case scanDoneMsg:
		return m.onScanned(msg)

	case planMsg:
		return m.onPlan(msg)

	case stepMsg:
		m.recordStep(msg.step)
		m.target = msg.fraction
		m.animating = true
		return m, waitFor(m.applyC)

	case applyMsg:
		m.result, m.err = msg.result, msg.err
		m.finished = time.Now()
		if msg.err != nil {
			m.screen = screenFailed
			if m.quitAfterApply {
				return m, tea.Quit
			}
			return m, nil
		}
		for i := range m.steps {
			m.steps[i].state = stepDone
		}
		m.spring.pos, m.spring.vel = 1, 0
		m.screen = screenDone
		proc.Invalidate()
		if m.quitAfterApply {
			return m, tea.Quit
		}
		return m, m.refreshRunning()

	case statusMsg:
		m.report, m.reportAt = msg.report, time.Now()
		m.running, m.runningAt = msg.report.Running, time.Now()
		return m, nil

	case doctorMsg:
		m.checks, m.scroll = msg.checks, 0
		return m, nil

	case runningMsg:
		m.running, m.runningAt = msg.status, time.Now()
		return m, nil

	case watchMsg:
		return m.onWatchEvent(msg)

	case clearMsg:
		return m.onClear(msg)

	case undoMsg:
		return m.onUndo(msg)

	case verifyMsg:
		return m.onVerify(msg)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyPressMsg:
		return m.onKey(msg)
	}

	return m, m.updateChild(msg)
}

func (m *Model) refreshRunning() tea.Cmd {
	if !m.showsRunning() {
		return nil
	}

	root := m.report.Helium.Root
	if root == "" {
		root = m.heliumDir
	}

	return func() tea.Msg {
		return runningMsg{status: proc.Current(proc.TTL).Helium(root)}
	}
}

func (m *Model) showsRunning() bool {
	switch m.screen {
	case screenPicking, screenStatus, screenDone, screenDoctor:
		return true
	}

	return false
}

func (m *Model) retheme() {
	m.spinner.Style = theme.Accent
	m.progress = progress.New(
		progress.WithWidth(clamp(m.layout().text(m.layout().content)-16, 8, 56)),
		progress.WithColors(theme.Primary, theme.Secondary, theme.Tertiary),
		progress.WithFillCharacters('█', '░'),
		progress.WithoutPercentage(),
	)

	m.helpCache = ""
	m.listCache, m.listCacheKey = "", listKey{}
	clearCaches()
}

func (m *Model) onScanned(msg scanDoneMsg) (tea.Model, tea.Cmd) {
	m.detections = msg.detections
	m.heliumDir, m.held = msg.helium, msg.held
	sort.Slice(m.detections, func(i, j int) bool {
		return m.detections[i].Priority < m.detections[j].Priority
	})

	items := make([]list.Item, 0, len(m.detections)+1)
	available := 0
	preselect := 0

	wanted := m.opts.Fixer.Browser
	if m.opts.Fixer.Fresh {
		wanted = browser.Fresh
	}
	if wanted == "" {
		wanted = m.last.id
	}

	m.hasFresh = m.opts.Fixer.SourcePath == ""

	if m.hasFresh {
		items = append(items, sourceItem{fresh: true})
		available++

		if wanted == browser.Fresh {
			preselect = 0
		}
	}

	for _, detection := range m.detections {
		if detection.Available() {
			if wanted != "" && detection.ID == wanted {
				preselect = len(items)
			}
			available++
		}
		items = append(items, sourceItem{detection: detection})
	}

	m.list.SetItems(items)

	if available == 0 {
		m.err = fixer.ErrNoSource
		m.screen = screenFailed
		return m, m.refreshRunning()
	}

	m.list.Select(preselect)
	m.history = nil
	m.screen = screenPicking

	return m, m.refreshRunning()
}

func (m *Model) onPlan(msg planMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.err
		m.screen = screenFailed
		return m, nil
	}

	m.plan = msg.plan
	m.planReady = true

	if !msg.apply {
		m.show(screenReview)
		return m, m.buildForm()
	}

	if m.dryRun {
		m.finished = time.Now()
		m.screen = screenDone
		return m, m.refreshRunning()
	}

	m.steps = nil
	m.started = time.Now()
	m.quitAfterApply = false
	m.notice = ""
	m.screen = screenApplying
	return m, m.startApply()
}

func (m *Model) startApply() tea.Cmd {
	ch := make(chan tea.Msg, 16)
	m.applyC = ch

	plan := m.plan
	logger := m.logger
	ctx := m.ctx

	go func() {
		defer close(ch)

		result, err := fixer.Apply(ctx, plan, logger, func(step string, fraction float64) {
			ch <- stepMsg{step: step, fraction: fraction}
		})
		ch <- applyMsg{result: result, err: err}
	}()

	return waitFor(ch)
}

func (m *Model) recordStep(name string) {
	if n := len(m.steps); n > 0 {
		if m.steps[n-1].name == name {
			return
		}
		m.steps[n-1].state = stepDone
	}

	m.steps = append(m.steps, applyStep{name: name, state: stepActive})
	m.step = name
}

func waitFor(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

func isBack(msg tea.KeyPressMsg) bool {
	switch msg.String() {
	case "esc", "ctrl+[", "escape":
		return true
	}

	return msg.Code == tea.KeyEscape || msg.Code == 0x1b || msg.Text == "\x1b"
}

func (m *Model) onKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if key == "ctrl+c" {
		if m.screen == screenApplying {
			if m.quitAfterApply {
				return m, tea.Quit
			}
			m.quitAfterApply = true
			m.setNotice("finishing the install, press ctrl+c again to quit now")
			return m, nil
		}
		return m, tea.Quit
	}

	if key == "ctrl+z" && m.screen != screenApplying && m.screen != screenReview {
		return m, tea.Suspend
	}

	switch m.screen {
	case screenScanning:
		if key == "q" || isBack(msg) {
			return m, tea.Quit
		}
		return m, nil

	case screenHelp:
		if isBack(msg) || key == "q" || key == "?" {
			m.back()
			return m, nil
		}

		return m, m.updateChild(msg)

	case screenPicking:
		return m.onPickKey(msg)

	case screenReview:
		return m.onReviewKey(msg)

	case screenApplying:
		return m, nil

	case screenStatus:
		return m.onStatusKey(msg)

	case screenDoctor:
		return m.onDoctorKey(msg)

	case screenWatch:
		return m.onWatchKey(msg)

	case screenClear:
		return m.onClearKey(msg)

	case screenDone, screenFailed:
		switch key {
		case "q", "enter":
			return m, tea.Quit
		case "r":
			m.reset()
			return m, tea.Batch(m.spinner.Tick, m.startScan())
		case "s":
			return m, m.openStatus()
		case "d":
			return m, m.openDoctor()
		case "w":
			return m, m.openWatch()
		case "v":
			if m.dryRun {
				m.setNotice("dry run, nothing was installed")
				return m, nil
			}
			return m, m.openStream()
		case "u":
			return m, m.undo()
		case "y":
			return m.confirmPlayback(true)
		case "n":
			return m.confirmPlayback(false)
		case "?":
			m.openHelp()
		}

		if isBack(msg) {
			m.backToPicker()
		}

		return m, nil
	}

	return m, nil
}

func (m *Model) onPickKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.list.FilterState() == list.Filtering {
		return m, m.updateChild(msg)
	}

	if isBack(msg) {
		return m, tea.Quit
	}

	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "f":
		m.opts.Fixer.Fresh = true
		m.planReady = false
		m.show(screenReview)
		return m, planCmd(m.ctx, m.opts.Fixer, m.logger, false)
	case "?":
		m.openHelp()
		return m, nil
	case "s":
		return m, m.openStatus()
	case "d":
		return m, m.openDoctor()
	case "w":
		return m, m.openWatch()
	case "enter":
		item, ok := m.list.SelectedItem().(sourceItem)
		if !ok || !item.usable() {
			return m, nil
		}

		m.opts.Fixer.Fresh = item.fresh
		if item.fresh {
			m.opts.Fixer.Browser = ""
		} else {
			m.opts.Fixer.Browser = item.detection.ID
		}

		m.planReady = false
		m.show(screenReview)
		return m, planCmd(m.ctx, m.opts.Fixer, m.logger, false)
	}

	return m, m.updateChild(msg)
}

func (m *Model) onReviewKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if isBack(msg) {
		m.form = nil
		m.planReady = false
		if !m.back() {
			m.screen = screenPicking
		}
		return m, nil
	}

	return m, m.updateChild(msg)
}

type charmTheme struct{}

func (charmTheme) Theme(bool) *huh.Styles {
	styles := huh.ThemeBase(true)

	styles.Group.Base = styles.Group.Base.Border(lipgloss.NormalBorder(), false, false, false, false)
	styles.Group.Title = lipgloss.NewStyle().Foreground(theme.Text).Bold(true)
	styles.Group.Description = lipgloss.NewStyle().Foreground(theme.Muted)

	styles.Focused = formFieldStyles(
		lipgloss.NewStyle().Foreground(theme.Primary).Bold(true),
		lipgloss.NewStyle().Foreground(theme.Muted),
		lipgloss.NewStyle().Foreground(theme.Text),
		lipgloss.NewStyle().Foreground(theme.Faint),
	)
	styles.Blurred = formFieldStyles(
		lipgloss.NewStyle().Foreground(theme.Muted),
		lipgloss.NewStyle().Foreground(theme.Faint),
		lipgloss.NewStyle().Foreground(theme.Muted),
		lipgloss.NewStyle().Foreground(theme.Faint),
	)

	return styles
}

func formFieldStyles(title, description, text, faint lipgloss.Style) huh.FieldStyles {
	var styles huh.FieldStyles

	styles.Base = lipgloss.NewStyle()
	styles.Title = title
	styles.Description = description
	styles.ErrorIndicator = lipgloss.NewStyle().Foreground(theme.Danger).SetString("!")
	styles.ErrorMessage = lipgloss.NewStyle().Foreground(theme.Danger)

	styles.SelectSelector = lipgloss.NewStyle().Foreground(theme.Primary).SetString("▌ ")
	styles.Option = text
	styles.NextIndicator = lipgloss.NewStyle().Foreground(theme.Muted).SetString("↓")
	styles.PrevIndicator = lipgloss.NewStyle().Foreground(theme.Muted).SetString("↑")

	styles.Directory = lipgloss.NewStyle().Foreground(theme.Primary)
	styles.File = text

	styles.MultiSelectSelector = lipgloss.NewStyle().Foreground(theme.Primary).SetString("▌ ")
	styles.SelectedOption = lipgloss.NewStyle().Foreground(theme.Primary)
	styles.SelectedPrefix = lipgloss.NewStyle().Foreground(theme.Primary).SetString("✓ ")
	styles.UnselectedOption = text
	styles.UnselectedPrefix = lipgloss.NewStyle().Foreground(theme.Faint).SetString("· ")

	styles.TextInput = huh.TextInputStyles{
		Cursor:      lipgloss.NewStyle().Foreground(theme.Ink).Background(theme.Primary),
		CursorText:  lipgloss.NewStyle().Foreground(theme.Ink).Background(theme.Primary),
		Placeholder: faint,
		Prompt:      lipgloss.NewStyle().Foreground(theme.Primary).SetString(""),
		Text:        text,
	}

	styles.FocusedButton = lipgloss.NewStyle().Foreground(theme.Ink).Background(theme.Primary).Padding(0, 2)
	styles.BlurredButton = lipgloss.NewStyle().Foreground(theme.Muted).Padding(0, 2)

	styles.Card = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(theme.Faint).Padding(0, 1)
	styles.NoteTitle = title
	styles.Next = lipgloss.NewStyle().Foreground(theme.Muted)

	return styles
}

func (m *Model) buildForm() tea.Cmd {
	m.heliumPath = m.opts.Fixer.HeliumPath
	m.dryRun = m.opts.DryRun

	l := m.layout()

	m.form = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Helium directory").
				Description("Leave empty to detect it automatically").
				Placeholder("auto-detect").
				Value(&m.heliumPath),
			huh.NewConfirm().
				Title("Dry run").
				Description("Resolve source and target without copying anything").
				Affirmative("yes").
				Negative("no").
				Value(&m.dryRun),
		),
	).WithTheme(charmTheme{}).WithWidth(formWidth(l)).WithShowHelp(false)

	return m.form.Init()
}

func (m *Model) updateChild(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	switch m.screen {
	case screenPicking:
		m.list, cmd = m.list.Update(msg)
	case screenReview:
		if m.form == nil {
			return nil
		}

		model, formCmd := m.form.Update(msg)
		if form, ok := model.(*huh.Form); ok {
			m.form = form
		}

		switch m.form.State {
		case huh.StateCompleted:
			changed := m.heliumPath != m.opts.Fixer.HeliumPath
			m.opts.Fixer.HeliumPath = m.heliumPath
			m.form = nil

			if changed {
				m.planReady = false
				return tea.Batch(formCmd, planCmd(m.ctx, m.opts.Fixer, m.logger, true))
			}

			return tea.Batch(formCmd, func() tea.Msg {
				return planMsg{plan: m.plan, apply: true}
			})
		case huh.StateAborted:
			m.form = nil
			if !m.back() {
				m.screen = screenPicking
			}
			m.planReady = false
			return formCmd
		}

		return formCmd
	case screenHelp:
		m.viewport, cmd = m.viewport.Update(msg)
	}

	return cmd
}

func (m *Model) show(next screen) {
	if m.screen == next {
		return
	}

	m.history = append(m.history, m.screen)
	if len(m.history) > 8 {
		m.history = m.history[len(m.history)-8:]
	}

	m.screen = next
}

func (m *Model) back() bool {
	if len(m.history) == 0 {
		return false
	}

	m.screen = m.history[len(m.history)-1]
	m.history = m.history[:len(m.history)-1]

	return true
}

func (m *Model) backToPicker() {
	m.form = nil
	m.planReady = false
	m.plan = fixer.Plan{}
	m.history = nil
	m.screen = screenPicking
}

func (m *Model) reset() {
	m.stopWatch()

	m.screen = screenScanning
	m.history = nil
	m.seen = make(map[browser.ID]browser.Detection, len(browser.All))
	m.scanned = 0
	m.detections = nil
	m.plan = fixer.Plan{}
	m.planReady = false
	m.result = fixer.Result{}
	m.err = nil
	m.steps = nil
	m.step = ""
	m.spring.pos, m.spring.vel, m.target = 0, 0, 0
	m.animating = false
	m.started = time.Now()
	m.finished = time.Time{}
	m.report = status.Report{}
	m.reportAt = time.Time{}
	m.checks = nil
	m.scroll = 0
	m.watchEvents = nil
	m.watchPaused = false
	m.watchScroll = 0
	m.clearCount = 0
	m.clearBytes = 0
	m.clearDir = ""
	m.clearStage = 0
	m.clearBusy = false
	m.verify = verifyNone
	m.undone = false
	m.quitAfterApply = false
	m.notice = ""
	m.noticeAt = time.Time{}
	m.list.SetItems(nil)
	m.listCache, m.listCacheKey = "", listKey{}
	m.hasFresh = false
}

func (m *Model) resize() {
	l := m.layout()

	inner := l.body
	if l.framed {
		inner -= frameHeight
	}
	inner = max(inner, 1)

	m.list.SetSize(max(l.text(l.left), 8), inner)
	m.progress.SetWidth(clamp(l.text(l.content)-16, 8, 56))
	m.viewport.SetWidth(clamp(l.text(l.content)-2, 8, 96))
	m.viewport.SetHeight(inner)
	m.helpCache = ""

	if m.form != nil {
		m.form.WithWidth(formWidth(l))
	}

	if m.screen == screenHelp {
		m.viewport.SetContent(m.renderHelp())
	}
}

func formWidth(l layout) int {
	if !l.detail {
		return clamp(l.text(l.content), 24, 88)
	}

	planWidth := l.content * 60 / 100

	return clamp(l.text(l.content-planWidth-l.gap), 24, 88)
}

func (m *Model) elapsed() string {
	end := m.finished
	if end.IsZero() {
		end = time.Now()
	}

	took := end.Sub(m.started)
	if took < time.Second {
		return fmt.Sprintf("%dms", took.Milliseconds())
	}

	return fmt.Sprintf("%.1fs", took.Seconds())
}
