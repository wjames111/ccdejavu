package cli

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/wjames111/ccdejavu/internal/links"
	"github.com/wjames111/ccdejavu/internal/syncer"
)

func newSyncCommand(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync chats across accounts once, now",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := g.paths()
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			st, err := syncer.Run(syncer.Options{Paths: p, DryRun: dryRun, Out: w, Now: time.Now})
			if err != nil {
				return err
			}
			printSummary(w, p, st, dryRun)
			l, err := links.Load(p.Links())
			if err != nil {
				return err
			}
			printIfNothingLinked(w, l)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print what would change and change nothing")
	return cmd
}
