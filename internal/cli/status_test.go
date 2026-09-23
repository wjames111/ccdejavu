package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/wjames111/ccdejavu/internal/layout"
	"github.com/wjames111/ccdejavu/internal/paths"
	"github.com/wjames111/ccdejavu/internal/state"
	"github.com/wjames111/ccdejavu/internal/testtree"
)

func TestStatusWithNoLinksListsUnlinkedAccounts(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	p := paths.Paths{Home: home}
	testtree.Mkdir(t, filepath.Join(p.CodeRoot(), testtree.AcctA, testtree.Org))
	testtree.Mkdir(t, filepath.Join(p.CodeRoot(), testtree.AcctB, testtree.Org))

	out, err := runCLI(t, "--home", home, "status")
	if err != nil {
		t.Fatal(err)
	}
	expectAll(t, out,
		"No linked accounts yet. Run: ccdejavu link <email> <email>",
		"Not linked: aaaaaaaa (email unknown), bbbbbbbb (email unknown)",
	)
}

func TestGroupResult(t *testing.T) {
	t.Parallel()
	p := paths.Paths{Home: "/h"}
	memberA, memberB := layout.Member(testtree.AcctA, testtree.Org), layout.Member(testtree.AcctB, testtree.Org)
	grp := layout.Group{
		Root:    layout.Root{Name: "code"},
		Name:    testtree.Org,
		Members: []string{memberA, memberB},
	}
	cases := []struct {
		name    string
		grp     layout.Group
		st      state.State
		want    string
		hasWant string // used when want is a prefix/substring check instead of exact
	}{
		{
			name: "only one account",
			grp:  layout.Group{Root: grp.Root, Name: testtree.Org, Members: []string{memberA}},
			st:   state.State{},
			want: "only one folder here, nothing to sync",
		},
		{
			name: "no record",
			grp:  grp,
			st:   state.State{},
			want: "not synced yet",
		},
		{
			name: "same accounts, no skip",
			grp:  grp,
			st: state.State{Groups: []state.Group{
				{Root: "code", Name: testtree.Org, Members: []string{memberA, memberB}, Actions: 2},
			}},
			want: "in sync (last sync made 2 changes)",
		},
		{
			name: "accounts differ",
			grp:  grp,
			st: state.State{Groups: []state.Group{
				{Root: "code", Name: testtree.Org, Members: []string{memberA}, Actions: 2},
			}},
			hasWant: "the folders changed",
		},
		{
			name: "skipped, no actions",
			grp:  grp,
			st: state.State{Groups: []state.Group{
				{Root: "code", Name: testtree.Org, Members: []string{memberA, memberB}, Skipped: "boom"},
			}},
			hasWant: "skipped last sync:",
		},
		{
			name: "skipped, partial",
			grp:  grp,
			st: state.State{Groups: []state.Group{
				{Root: "code", Name: testtree.Org, Members: []string{memberA, memberB}, Actions: 3, Skipped: "boom"},
			}},
			hasWant: "partly synced last time (3 changes made)",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := groupResult(p, c.grp, c.st)
			if c.want != "" && got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
			if c.hasWant != "" && !strings.HasPrefix(got, c.hasWant) && !strings.Contains(got, c.hasWant) {
				t.Errorf("got %q, want it to contain %q", got, c.hasWant)
			}
		})
	}
}
