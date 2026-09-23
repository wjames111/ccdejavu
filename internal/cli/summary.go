package cli

import (
	"fmt"
	"io"

	"github.com/wjames111/ccdejavu/internal/links"
	"github.com/wjames111/ccdejavu/internal/paths"
	"github.com/wjames111/ccdejavu/internal/state"
)

// printIfNothingLinked prints a hint when there's nothing to sync yet.
func printIfNothingLinked(w io.Writer, l links.Links) {
	if len(l.Groups) == 0 {
		fmt.Fprintln(w, "No accounts are linked yet, so nothing synced. Run: ccdejavu link <email> <email>")
	}
}

// printSummary prints one line per group that has more than one folder.
func printSummary(w io.Writer, p paths.Paths, st state.State, dryRun bool) {
	noun := "change"
	if dryRun {
		noun = "planned change"
	}
	for _, g := range st.Groups {
		if len(g.Members) < 2 {
			continue
		}
		heading := fmt.Sprintf("%s · %s", g.Root, g.Name)
		if g.Skipped != "" && g.Actions > 0 {
			fmt.Fprintf(w, "%s: partly synced (%s made, then stopped): %s\n",
				heading, plural(g.Actions, "change"), p.Brief(g.Skipped))
			continue
		}
		if g.Skipped != "" {
			fmt.Fprintf(w, "%s: skipped (%s)\n", heading, p.Brief(g.Skipped))
			continue
		}
		fmt.Fprintf(w, "%s: %s across %d folders\n", heading, plural(g.Actions, noun), len(g.Members))
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
