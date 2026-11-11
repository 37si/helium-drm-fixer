package watch

import (
	"context"
	"fmt"
	"time"

	"charm.land/log/v2"

	"hdf/internal/fixer"
	"hdf/internal/helium"
	"hdf/internal/units"
)

type Kind int

const (
	KindArmed Kind = iota
	KindChecked
	KindApplied
	KindError
)

type Event struct {
	At      time.Time
	Kind    Kind
	Message string
	Version string
}

type Options struct {
	Interval time.Duration
	Fixer    fixer.Options
	DryRun   bool
}

type Watcher struct {
	opts   Options
	logger *log.Logger
	last   string
	dir    string
}

const MinInterval = 5 * time.Second

func New(opts Options, logger *log.Logger) *Watcher {
	if opts.Interval < MinInterval {
		opts.Interval = MinInterval
	}

	return &Watcher{opts: opts, logger: logger}
}

func (w *Watcher) Run(ctx context.Context) <-chan Event {
	events := make(chan Event, 8)

	go func() {
		defer close(events)

		events <- Event{
			At:      time.Now(),
			Kind:    KindArmed,
			Message: "watching every " + units.Duration(w.opts.Interval),
		}

		ticker := time.NewTicker(w.opts.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				event, ok := w.tick(ctx)
				if !ok {
					continue
				}

				select {
				case events <- event:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return events
}

func (w *Watcher) tick(ctx context.Context) (Event, bool) {
	plan, err := fixer.Resolve(ctx, w.opts.Fixer, w.logger, nil)
	if err != nil {
		return w.once(KindError, "no source: "+err.Error(), "")
	}

	if plan.Helium.Dir != w.dir {
		w.dir = plan.Helium.Dir
		w.last = ""
	}

	current := installed(plan.Helium)

	switch {
	case current == plan.Source.Version:
		return w.once(KindChecked, fmt.Sprintf("up to date with %s %s", plan.Source.Name, current), current)
	case current == "":
		return w.install(ctx, plan, "Helium has no WidevineCdm")
	default:
		return w.install(ctx, plan, fmt.Sprintf("%s is installed, %s is available", current, plan.Source.Version))
	}
}

func (w *Watcher) install(ctx context.Context, plan fixer.Plan, reason string) (Event, bool) {
	if w.opts.DryRun {
		return w.once(KindApplied, reason+" (dry run, nothing written)", plan.Source.Version)
	}

	result, err := fixer.Apply(ctx, plan, w.logger, nil)
	if err != nil {
		return w.once(KindError, "install failed: "+err.Error(), "")
	}

	w.last = ""

	return Event{
		At:      time.Now(),
		Kind:    KindApplied,
		Version: result.Version,
		Message: fmt.Sprintf("installed %s from %s, %s", result.Version, plan.Source.Name, reason),
	}, true
}

func (w *Watcher) once(kind Kind, message, version string) (Event, bool) {
	if message == w.last {
		return Event{}, false
	}
	w.last = message

	return Event{At: time.Now(), Kind: kind, Message: message, Version: version}, true
}

func installed(install helium.Install) string {
	found := helium.Installed(install)
	if len(found) == 0 {
		return ""
	}

	return found[0].Version
}
