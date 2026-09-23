package watch_test

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/wjames111/ccdejavu/internal/paths"
	"github.com/wjames111/ccdejavu/internal/testtree"
	"github.com/wjames111/ccdejavu/internal/watch"
)

func wait(t *testing.T, calls <-chan struct{}) {
	t.Helper()
	select {
	case <-calls:
	case <-time.After(5 * time.Second):
		t.Fatal("sync was not called")
	}
}

func expectQuiet(t *testing.T, calls <-chan struct{}, d time.Duration) {
	t.Helper()
	select {
	case <-calls:
		t.Fatal("sync was called when it shouldn't have been")
	case <-time.After(d):
	}
}

func TestLoopSyncsAtStartAfterQuietAndOnTick(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	events := make(chan struct{})
	tick := make(chan time.Time)
	calls := make(chan struct{}, 10)
	done := make(chan struct{})
	go func() {
		watch.Loop(ctx, events, tick, 100*time.Millisecond, time.Hour, func() { calls <- struct{}{} })
		close(done)
	}()

	wait(t, calls) // at start
	for range 5 {
		events <- struct{}{}
	}
	wait(t, calls) // once, after the burst goes quiet
	expectQuiet(t, calls, 300*time.Millisecond)
	tick <- time.Now()
	wait(t, calls)

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Loop did not return after cancel")
	}
}

func TestRunSyncsWhenAnOrgFolderChanges(t *testing.T) {
	t.Parallel()
	p := paths.Paths{Home: t.TempDir()}
	org := filepath.Join(p.CodeRoot(), testtree.AcctA, testtree.Org)
	testtree.Mkdir(t, org)
	calls := make(chan struct{}, 10)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := watch.Config{Debounce: 50 * time.Millisecond, MaxWait: time.Second, Interval: time.Hour}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- watch.Run(ctx, p, cfg, log, func() { calls <- struct{}{} })
	}()

	wait(t, calls) // at start, after watches are set
	testtree.Write(t, filepath.Join(org, "local_"+testtree.Chat1+".json"), testtree.ChatJSON(testtree.Chat1, "new"), testtree.T0)
	wait(t, calls)

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}

func TestRunWatchesOrgsThatAppearLater(t *testing.T) {
	t.Parallel()
	p := paths.Paths{Home: t.TempDir()}
	testtree.Mkdir(t, p.CodeRoot())
	calls := make(chan struct{}, 10)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := watch.Config{Debounce: 50 * time.Millisecond, MaxWait: time.Second, Interval: time.Hour}
	go func() {
		_ = watch.Run(t.Context(), p, cfg, log, func() { calls <- struct{}{} })
	}()

	wait(t, calls) // at start
	org := filepath.Join(p.CodeRoot(), testtree.AcctA, testtree.Org)
	testtree.Mkdir(t, org)
	wait(t, calls) // the root sees the new account folder
	drain(calls, 300*time.Millisecond)
	testtree.Write(t, filepath.Join(org, "local_"+testtree.Chat1+".json"), testtree.ChatJSON(testtree.Chat1, "new"), testtree.T0)
	wait(t, calls) // the new org folder is watched now
}

// drain discards calls until none arrive for quiet.
func drain(calls <-chan struct{}, quiet time.Duration) {
	for {
		select {
		case <-calls:
		case <-time.After(quiet):
			return
		}
	}
}

func TestLoopSyncsWithinMaxWaitDuringSteadyChanges(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	events := make(chan struct{})
	calls := make(chan struct{}, 10)
	go watch.Loop(ctx, events, nil, 50*time.Millisecond, 200*time.Millisecond, func() { calls <- struct{}{} })
	wait(t, calls) // at start
	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				select {
				case events <- struct{}{}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	select {
	case <-calls:
	case <-time.After(time.Second):
		t.Fatal("steady changes held off the sync past the max wait")
	}
}
