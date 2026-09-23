package cli

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/wjames111/ccdejavu/internal/launchd"
	"github.com/wjames111/ccdejavu/internal/syncer"
)

func newInstallCommand(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Back up, sync once, and start the background job",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if g.home != "" {
				return errors.New("install doesn't support --home; it always manages your real background job")
			}
			p, err := g.paths()
			if err != nil {
				return err
			}
			bin, err := os.Executable()
			if err != nil {
				return err
			}
			if launchd.IsTransientPath(bin) {
				return fmt.Errorf("refusing to install from a temporary build (%s): build a real binary with `task build` and run ./ccdejavu install", bin)
			}
			w := cmd.OutOrStdout()
			st, err := syncer.Run(syncer.Options{Paths: p, Out: w, Now: time.Now})
			if err != nil {
				return err
			}
			printSummary(w, p, st, false)
			if err := launchd.Install(cmd.Context(), p, bin); err != nil {
				return fmt.Errorf("chats synced, but the background job didn't start: %w", err)
			}
			fmt.Fprintf(w, "\nInstalled %s\nccdejavu now keeps your chats in sync in the background. Logs: %s/\n",
				launchd.DefinitionPath(p), p.Logs())
			fmt.Fprintln(w, "Restart the Claude desktop app once so it shows the synced chats.")
			return nil
		},
	}
}

func newUninstallCommand(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Stop and remove the background job (your chats stay put)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if g.home != "" {
				return errors.New("uninstall doesn't support --home; it always manages your real background job")
			}
			p, err := g.paths()
			if err != nil {
				return err
			}
			if err := launchd.Uninstall(cmd.Context(), p); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Removed the background job. Your chats are left where they are.")
			return nil
		},
	}
}
