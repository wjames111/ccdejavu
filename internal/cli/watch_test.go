package cli

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/wjames111/ccdejavu/internal/paths"
	"github.com/wjames111/ccdejavu/internal/state"
	"github.com/wjames111/ccdejavu/internal/testtree"
)

func TestSyncLogReportLogsAStuckGroupOnce(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	s := &syncLog{p: paths.Paths{Home: "/h"}, log: log, last: map[string]string{}}
	skipped := []state.Group{{Root: "code", Name: testtree.Org, Skipped: "boom"}}

	s.report(skipped)
	s.report(skipped)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	skippedLines, stillSkippedLines := 0, 0
	for _, l := range lines {
		switch {
		case strings.Contains(l, "msg=skipped"):
			skippedLines++
		case strings.Contains(l, "msg=\"still skipped\""):
			stillSkippedLines++
		}
	}
	if skippedLines != 1 {
		t.Errorf("got %d skipped lines, want 1:\n%s", skippedLines, buf.String())
	}
	if stillSkippedLines != 1 {
		t.Errorf("got %d still-skipped lines, want 1:\n%s", stillSkippedLines, buf.String())
	}

	buf.Reset()
	s.report([]state.Group{{Root: "code", Name: testtree.Org, Actions: 1}})
	if !strings.Contains(buf.String(), "msg=recovered") {
		t.Errorf("output = %q, want a recovered line", buf.String())
	}
}

func TestSyncLogRunLogsFailureOnceThenDebug(t *testing.T) {
	t.Parallel()
	p := paths.Paths{Home: t.TempDir()}
	testtree.Mkdir(t, p.State())
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	s := &syncLog{p: p, log: log, last: map[string]string{}}

	s.run()
	s.run()

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	failed, stillFailing := 0, 0
	for _, l := range lines {
		switch {
		case strings.Contains(l, `msg="sync failed"`):
			failed++
		case strings.Contains(l, `msg="still failing"`):
			stillFailing++
		}
	}
	if failed != 1 {
		t.Errorf("got %d sync failed lines, want 1:\n%s", failed, buf.String())
	}
	if stillFailing != 1 {
		t.Errorf("got %d still failing lines, want 1:\n%s", stillFailing, buf.String())
	}
}

func TestWatchRejectsAnUnknownLogLevel(t *testing.T) {
	t.Parallel()
	_, err := runCLI(t, "--home", t.TempDir(), "--log-level", "loud", "watch")
	if err == nil || !strings.Contains(err.Error(), "--log-level") {
		t.Fatalf("err = %v, want it to mention --log-level", err)
	}
}
