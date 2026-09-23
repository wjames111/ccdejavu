package cli

import (
	"fmt"
	"io"

	"github.com/wjames111/ccdejavu/internal/paths"
	"github.com/wjames111/ccdejavu/internal/state"
)

// printSummary prints one line per group that has more than one account.
func printSummary(w io.Writer, p paths.Paths, st state.State, dryRun bool) {
	noun := "change"
	if dryRun {
		noun = "planned change"
	}
	for _, g := range st.Groups {
		if len(g.Accounts) < 2 {
			continue
		}
		if g.Skipped != "" && g.Actions > 0 {
			fmt.Fprintf(w, "%s org %s: partly synced (%s made, then stopped): %s\n",
				g.Root, short(g.Org), plural(g.Actions, "change"), p.Brief(g.Skipped))
			continue
		}
		if g.Skipped != "" {
			fmt.Fprintf(w, "%s org %s: skipped (%s)\n", g.Root, short(g.Org), p.Brief(g.Skipped))
			continue
		}
		fmt.Fprintf(w, "%s org %s: %s across %d accounts\n", g.Root, short(g.Org), plural(g.Actions, noun), len(g.Accounts))
	}
}

// short trims a UUID to the 8 characters people can tell apart at a glance.
func short(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
