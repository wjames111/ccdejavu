// Package watch re-runs the sync whenever the app's chat folders change.
package watch

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/wjames111/ccdejavu/internal/layout"
	"github.com/wjames111/ccdejavu/internal/paths"
)

// Config sets how quickly the watcher reacts.
type Config struct {
	Debounce time.Duration // quiet time after a change before syncing
	MaxWait  time.Duration // longest a sync waits once changes start, even if they keep coming
	Interval time.Duration // full sync regardless of changes
}

// Loop calls sync once at start, again once events have been quiet for
// debounce (or maxWait after the first unsynced event, whichever comes
// first), and on every tick. It returns when ctx is done.
func Loop(ctx context.Context, events <-chan struct{}, tick <-chan time.Time, debounce, maxWait time.Duration, sync func()) {
	sync()
	timer := time.NewTimer(debounce)
	timer.Stop()
	var fire <-chan time.Time
	var deadline time.Time
	for {
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-events:
			if fire == nil {
				deadline = time.Now().Add(maxWait)
			}
			timer.Reset(min(debounce, time.Until(deadline)))
			fire = timer.C
		case <-fire:
			fire = nil
			sync()
		case <-tick:
			timer.Stop()
			fire = nil
			sync()
		}
	}
}

// Run watches the app's folders and drives Loop until ctx is done.
func Run(ctx context.Context, p paths.Paths, cfg Config, log *slog.Logger, sync func()) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()

	changed := make(chan struct{}, 1)
	go forward(w, changed, log)

	addWatches(w, p, log)
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()
	Loop(ctx, changed, ticker.C, cfg.Debounce, cfg.MaxWait, func() {
		sync()
		addWatches(w, p, log) // pick up accounts and orgs that appeared since
	})
	return nil
}

// forward turns fsnotify events into a single "something changed" signal.
func forward(w *fsnotify.Watcher, changed chan<- struct{}, log *slog.Logger) {
	for {
		select {
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			log.Debug("change", "path", ev.Name, "op", ev.Op.String())
			select {
			case changed <- struct{}{}:
			default:
			}
		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			log.Warn("watch error", "err", err)
		}
	}
}

// addWatches watches both roots, every account folder, and every org folder.
// Folders inside Cowork chats aren't watched: kqueue holds one open file per
// watched file, and the periodic pass covers them.
func addWatches(w *fsnotify.Watcher, p paths.Paths, log *slog.Logger) {
	roots := layout.Roots(p)
	var dirs []string
	for _, r := range roots {
		dirs = append(dirs, r.Dir)
	}
	groups, err := layout.Discover(roots)
	if err != nil {
		log.Warn("discover failed", "err", err)
	}
	for _, g := range groups {
		for _, a := range g.Accounts {
			dirs = append(dirs, filepath.Join(g.Root.Dir, a), g.OrgDir(a))
		}
	}
	for _, d := range dirs {
		if err := w.Add(d); err != nil && !errors.Is(err, fs.ErrNotExist) {
			log.Warn("watch failed", "dir", d, "err", err)
		}
	}
}
