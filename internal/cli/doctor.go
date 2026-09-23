package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/wjames111/ccdejavu/internal/launchd"
	"github.com/wjames111/ccdejavu/internal/layout"
	"github.com/wjames111/ccdejavu/internal/paths"
)

type check struct {
	name string
	err  error
}

func newDoctorCommand(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check the app's folders and the background job",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := g.paths()
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			failed := 0
			for _, c := range runChecks(cmd.Context(), p) {
				if c.err != nil {
					failed++
					fmt.Fprintf(w, "FAIL %s: %v\n", c.name, c.err)
				} else {
					fmt.Fprintf(w, "ok   %s\n", c.name)
				}
			}
			if failed > 0 {
				return fmt.Errorf("doctor found %s", plural(failed, "problem"))
			}
			return nil
		},
	}
}

func runChecks(ctx context.Context, p paths.Paths) []check {
	_, err := os.Stat(p.AppSupport())
	if errors.Is(err, fs.ErrNotExist) {
		err = fmt.Errorf("not found at %s; is the Claude desktop app installed?", p.AppSupport())
	}
	checks := []check{{"Claude desktop app folder", err}}

	groups, err := layout.Discover(layout.Roots(p))
	if err == nil && len(groups) == 0 {
		err = errors.New("no Code tab or Cowork account folders found")
	}
	checks = append(checks, check{"chat folders", err})
	for _, grp := range groups {
		if grp.Syncable() {
			name := fmt.Sprintf("%s org %s layout", grp.Root.Name, short(grp.Org))
			checks = append(checks, check{name, layout.Validate(grp)})
		}
	}
	return append(checks, check{"background job", jobCheck(ctx, p)})
}

func jobCheck(ctx context.Context, p paths.Paths) error {
	installed, err := launchd.InstalledProgram(p)
	if errors.Is(err, fs.ErrNotExist) {
		return errors.New("not installed; run: ccdejavu install")
	}
	if err != nil {
		return err
	}
	if _, err := os.Stat(installed); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("it runs %s, which no longer exists; re-run: ccdejavu install", installed)
	} else if err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if !samePath(installed, self) {
		return fmt.Errorf("it runs %s but this is %s; re-run: ccdejavu install", installed, self)
	}
	if !launchd.Running(ctx) {
		return fmt.Errorf("installed but not running; see %s/", p.Logs())
	}
	return nil
}

func samePath(a, b string) bool {
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	if errA != nil || errB != nil {
		return a == b
	}
	return ra == rb
}
