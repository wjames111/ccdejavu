package paths_test

import (
	"testing"

	"github.com/wjames111/ccdejavu/internal/paths"
)

func TestPathsAreRootedAtHome(t *testing.T) {
	t.Parallel()
	p := paths.Paths{Home: "/h"}
	cases := []struct{ got, want string }{
		{p.AppSupport(), "/h/Library/Application Support/Claude"},
		{p.CodeRoot(), "/h/Library/Application Support/Claude/claude-code-sessions"},
		{p.CoworkRoot(), "/h/Library/Application Support/Claude/local-agent-mode-sessions"},
		{p.Data(), "/h/.ccdejavu"},
		{p.Backups(), "/h/.ccdejavu/backups"},
		{p.Trash(), "/h/.ccdejavu/trash"},
		{p.State(), "/h/.ccdejavu/state.json"},
		{p.Lock(), "/h/.ccdejavu/lock"},
		{p.Links(), "/h/.ccdejavu/links.json"},
		{p.ClaudeConfig(), "/h/.claude.json"},
		{p.Logs(), "/h/Library/Logs/ccdejavu"},
		{p.LaunchAgents(), "/h/Library/LaunchAgents"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}

func TestBriefShortensAppAndHomePaths(t *testing.T) {
	t.Parallel()
	p := paths.Paths{Home: "/h"}
	reason := p.AppSupport() + "/claude-code-sessions/x: " + p.Home + "/.ccdejavu/trash"
	got := p.Brief(reason)
	want := "claude-code-sessions/x: ~/.ccdejavu/trash"
	if got != want {
		t.Errorf("Brief = %q, want %q", got, want)
	}
}
