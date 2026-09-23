package cli

import (
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/wjames111/ccdejavu/internal/paths"
	"github.com/wjames111/ccdejavu/internal/state"
	"github.com/wjames111/ccdejavu/internal/syncer"
	"github.com/wjames111/ccdejavu/internal/watch"
)

func newWatchCommand(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "watch",
		Short: "Keep syncing as chats change (what the background job runs)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := g.paths()
			if err != nil {
				return err
			}
			log, err := g.logger(cmd.OutOrStdout())
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			log.Info("watching for chat changes", "code", p.CodeRoot(), "cowork", p.CoworkRoot())
			cfg := watch.Config{Debounce: 2 * time.Second, MaxWait: 30 * time.Second, Interval: 5 * time.Minute}
			sl := &syncLog{p: p, log: log, last: map[string]string{}}
			err = watch.Run(ctx, p, cfg, log, sl.run)
			log.Info("stopped")
			return err
		},
	}
}

// syncLog remembers each group's last skip reason so a stuck group is logged once, not every pass.
type syncLog struct {
	p       paths.Paths
	log     *slog.Logger
	last    map[string]string
	lastErr string
}

func (s *syncLog) run() {
	st, err := syncer.Run(syncer.Options{Paths: s.p, Out: io.Discard, Now: time.Now})
	if err != nil {
		if err.Error() != s.lastErr {
			s.log.Error("sync failed", "err", err)
			s.lastErr = err.Error()
		} else {
			s.log.Debug("still failing", "err", err)
		}
		return
	}
	if s.lastErr != "" {
		s.log.Info("sync recovered")
		s.lastErr = ""
	}
	s.report(st.Groups)
}

func (s *syncLog) report(groups []state.Group) {
	for _, grp := range groups {
		key := grp.Root + " " + grp.Name
		wasSkipped := s.last[key] != ""
		switch {
		case grp.Skipped != "" && grp.Skipped != s.last[key]:
			s.log.Warn("skipped", "root", grp.Root, "group", grp.Name, "reason", s.p.Brief(grp.Skipped))
		case grp.Skipped != "":
			s.log.Debug("still skipped", "root", grp.Root, "group", grp.Name, "reason", s.p.Brief(grp.Skipped))
		case wasSkipped:
			s.log.Info("recovered", "root", grp.Root, "group", grp.Name)
		case grp.Actions > 0:
			s.log.Info("synced", "root", grp.Root, "group", grp.Name, "changes", grp.Actions)
		}
		s.last[key] = grp.Skipped
	}
}
