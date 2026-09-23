// Package syncer runs one full sync: back up on the first run, then validate,
// plan, and apply every group, and record what happened.
package syncer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/wjames111/ccdejavu/internal/apply"
	"github.com/wjames111/ccdejavu/internal/backup"
	"github.com/wjames111/ccdejavu/internal/layout"
	"github.com/wjames111/ccdejavu/internal/links"
	"github.com/wjames111/ccdejavu/internal/paths"
	"github.com/wjames111/ccdejavu/internal/plan"
	"github.com/wjames111/ccdejavu/internal/state"
)

// Options configures one sync.
type Options struct {
	Paths  paths.Paths
	DryRun bool      // print the plan, change nothing
	Out    io.Writer // backup notices and dry-run plans
	Now    func() time.Time
}

// Run syncs every group once. A group that fails validation or apply is
// recorded as skipped and the rest still sync. A failed first-run backup
// stops everything.
func Run(opts Options) (state.State, error) {
	p := opts.Paths
	roots := layout.Roots(p)

	if !opts.DryRun {
		unlock, err := Lock(p.Lock())
		if err != nil {
			return state.State{}, err
		}
		defer unlock()
	}

	st, err := state.Load(p.State())
	if err != nil {
		return st, fmt.Errorf("reading %s: %w", p.State(), err)
	}
	l, err := links.Load(p.Links())
	if err != nil {
		return st, err
	}
	groups, err := layout.Groups(roots, l.Sets())
	if err != nil {
		return st, err
	}

	stamp := opts.Now().UTC().Format("20060102-150405")
	anySyncable := slices.ContainsFunc(groups, layout.Group.Syncable)
	newSyncable := slices.ContainsFunc(groups, func(g layout.Group) bool {
		return g.Syncable() && !recorded(st, g)
	})
	if anySyncable && (st.Backup == "" || newSyncable) {
		dest := filepath.Join(p.Backups(), stamp)
		if opts.DryRun {
			fmt.Fprintf(opts.Out, "Would back up both folders to %s first\n", dest)
		} else {
			fmt.Fprintf(opts.Out, "Backing up both folders to %s first (this can take a minute)\n", dest)
			if err := takeBackup(roots, dest); err != nil {
				return st, fmt.Errorf("first-run backup failed, nothing was synced: %w", err)
			}
			st.Backup = dest
			if err := state.Save(p.State(), st); err != nil {
				return st, err
			}
			fmt.Fprintf(opts.Out, "Backed up both folders to %s\n", dest)
		}
	}

	trashRoot := filepath.Join(p.Trash(), stamp)
	st.Groups = nil
	for _, g := range groups {
		rec := state.Group{Root: g.Root.Name, Name: g.Name, Members: g.Members}
		if g.Syncable() {
			n, err := syncGroup(g, trashRoot, opts)
			rec.Actions = n
			if err != nil {
				rec.Skipped = err.Error()
			}
		}
		st.Groups = append(st.Groups, rec)
	}

	if opts.DryRun {
		return st, nil
	}
	st.LastSync = opts.Now()
	return st, state.Save(p.State(), st)
}

func syncGroup(g layout.Group, trashRoot string, opts Options) (int, error) {
	if err := layout.Validate(g); err != nil {
		return 0, err
	}
	actions, err := plan.Build(g, trashRoot)
	if err != nil {
		return 0, err
	}
	if opts.DryRun {
		for _, a := range actions {
			fmt.Fprintln(opts.Out, opts.Paths.Brief(a.String()))
		}
		return len(actions), nil
	}
	done, err := apply.Run(actions)
	if err != nil {
		return done, fmt.Errorf("stopped after %d of %d changes: %w", done, len(actions), err)
	}
	return done, nil
}

// recorded reports whether g's exact member set was already recorded in a previous run.
func recorded(st state.State, g layout.Group) bool {
	return slices.ContainsFunc(st.Groups, func(r state.Group) bool {
		return r.Root == g.Root.Name && slices.Equal(r.Members, g.Members)
	})
}

// takeBackup renames the copy into place only when it's complete, so a failed backup never looks real.
func takeBackup(roots []layout.Root, dest string) error {
	partial := dest + ".partial"
	if err := os.MkdirAll(partial, 0o700); err != nil {
		return err
	}
	if err := backup.Run(roots, partial); err != nil {
		os.RemoveAll(partial)
		return err
	}
	return os.Rename(partial, dest)
}
