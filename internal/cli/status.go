package cli

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/wjames111/ccdejavu/internal/identity"
	"github.com/wjames111/ccdejavu/internal/launchd"
	"github.com/wjames111/ccdejavu/internal/layout"
	"github.com/wjames111/ccdejavu/internal/links"
	"github.com/wjames111/ccdejavu/internal/paths"
	"github.com/wjames111/ccdejavu/internal/state"
)

func newStatusCommand(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show accounts, chat counts, and the last sync",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := g.paths()
			if err != nil {
				return err
			}
			st, err := state.Load(p.State())
			if err != nil {
				return err
			}
			l, err := links.Load(p.Links())
			if err != nil {
				return err
			}
			roots := layout.Roots(p)
			groups, err := layout.Groups(roots, l.Sets())
			if err != nil {
				return err
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%-16s%s\n", "Last sync:", when(st.LastSync))
			fmt.Fprintf(w, "%-16s%s\n", "Background job:", jobState(cmd.Context(), p))
			fmt.Fprintf(w, "%-16s%s\n", "Backup:", orNone(st.Backup))
			fmt.Fprintf(w, "%-16s%s\n", "Trash:", humanBytes(dirSize(p.Trash())))

			if len(l.Groups) == 0 {
				fmt.Fprint(w, "\nNo linked accounts yet. Run: ccdejavu link <email> <email>\n")
			}
			for _, grp := range groups {
				fmt.Fprintf(w, "\n%s · %s\n", grp.Root.Name, grp.Name)
				for _, m := range grp.Members {
					n, err := layout.CountChats(grp.Dir(m))
					if err != nil {
						return err
					}
					acct, org, _ := strings.Cut(m, "/")
					fmt.Fprintf(w, "  %s · org %s  %s\n", l.EmailFor(acct), short(org), plural(n, "chat"))
				}
				fmt.Fprintf(w, "  %s\n", groupResult(p, grp, st))
			}

			unlinked, err := notLinked(p, roots, l)
			if err != nil {
				return err
			}
			if unlinked != "" {
				fmt.Fprintf(w, "\nNot linked: %s\n", unlinked)
			}
			return nil
		},
	}
}

// notLinked lists, as "<id8> (<email>)" pairs, the accounts found on this Mac that aren't in any link.
func notLinked(p paths.Paths, roots []layout.Root, l links.Links) (string, error) {
	found := map[string]bool{}
	for _, r := range roots {
		folders, err := layout.AccountFolders(r)
		if err != nil {
			return "", err
		}
		for a := range folders {
			if l.EmailFor(a) == "" {
				found[a] = true
			}
		}
	}
	if len(found) == 0 {
		return "", nil
	}
	ids := make([]string, 0, len(found))
	for a := range found {
		ids = append(ids, a)
	}
	sort.Strings(ids)
	known := identity.Known(p)
	parts := make([]string, len(ids))
	for i, id := range ids {
		email := known[id]
		if email == "" {
			email = "email unknown"
		}
		parts[i] = fmt.Sprintf("%s (%s)", short(id), email)
	}
	return strings.Join(parts, ", "), nil
}

func when(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

func orNone(s string) string {
	if s == "" {
		return "none yet"
	}
	return s
}

func jobState(ctx context.Context, p paths.Paths) string {
	if _, err := os.Stat(launchd.DefinitionPath(p)); err != nil {
		return "not installed (run: ccdejavu install)"
	}
	if launchd.Running(ctx) {
		return "running"
	}
	return "installed but not running (see " + p.Logs() + "/)"
}

func groupResult(p paths.Paths, grp layout.Group, st state.State) string {
	if !grp.Syncable() {
		return "only one account here, nothing to sync"
	}
	for _, r := range st.Groups {
		if r.Root == grp.Root.Name && slices.Equal(r.Members, grp.Members) {
			if r.Skipped != "" && r.Actions > 0 {
				return fmt.Sprintf("partly synced last time (%s made): %s", plural(r.Actions, "change"), p.Brief(r.Skipped))
			}
			if r.Skipped != "" {
				return "skipped last sync: " + p.Brief(r.Skipped)
			}
			return fmt.Sprintf("in sync (last sync made %s)", plural(r.Actions, "change"))
		}
		if r.Root == grp.Root.Name && r.Name == grp.Name {
			return "a folder was added since the last sync; it syncs on the next pass"
		}
	}
	return "not synced yet"
}

func dirSize(dir string) int64 {
	var total int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
