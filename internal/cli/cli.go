// Package cli implements the ccdejavu command tree.
package cli

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/wjames111/ccdejavu/internal/build"
	"github.com/wjames111/ccdejavu/internal/paths"
)

type globals struct {
	logLevel string
	home     string
}

// Main runs the CLI and exits with a non-zero status on error.
func Main() {
	if err := newRootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	g := &globals{}
	root := &cobra.Command{
		Use:     "ccdejavu",
		Short:   "Keep Claude desktop app chats in sync across accounts",
		Version: build.String(),
		Long: "ccdejavu keeps the Claude desktop app's Code tab and Cowork chats in sync\n" +
			"across the Claude accounts you link on this Mac, so switching accounts\n" +
			"doesn't hide your chats. Nothing syncs until you link accounts with `ccdejavu link`.",
		SilenceUsage: true,
	}
	root.PersistentFlags().StringVar(&g.logLevel, "log-level", "info", "log verbosity: debug, info, warn, error")
	root.PersistentFlags().StringVar(&g.home, "home", "", "act on this home folder instead of yours (for tests)")
	_ = root.PersistentFlags().MarkHidden("home")
	root.AddCommand(
		newInstallCommand(g),
		newUninstallCommand(g),
		newSyncCommand(g),
		newWatchCommand(g),
		newStatusCommand(g),
		newDoctorCommand(g),
		newLinkCommand(g),
		newUnlinkCommand(g),
	)
	return root
}

func (g *globals) paths() (paths.Paths, error) {
	if g.home != "" {
		return paths.Paths{Home: g.home}, nil
	}
	return paths.Default()
}

func (g *globals) logger(w io.Writer) (*slog.Logger, error) {
	var level slog.Level
	switch g.logLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		return nil, fmt.Errorf("unknown --log-level %q (use debug, info, warn, or error)", g.logLevel)
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})), nil
}
