package identity_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/wjames111/ccdejavu/internal/identity"
	"github.com/wjames111/ccdejavu/internal/paths"
	"github.com/wjames111/ccdejavu/internal/testtree"
)

func oauth(id, email string) string {
	return `{"oauthAccount":{"accountUuid":"` + id + `","emailAddress":"` + email + `"}}`
}

func TestKnownReadsCoworkChatsAndCLIConfig(t *testing.T) {
	t.Parallel()
	p := paths.Paths{Home: t.TempDir()}
	// A's pair sits in B's folder: keyed by the ID inside, wherever the file sits.
	testtree.Write(t, filepath.Join(p.CoworkRoot(), testtree.AcctB, testtree.Org, "local_"+testtree.Chat1, ".claude", ".claude.json"), oauth(testtree.AcctA, "A@x.com"), testtree.T0)
	testtree.Write(t, p.ClaudeConfig(), oauth(testtree.AcctB, "b@y.com"), testtree.T0)
	testtree.Write(t, filepath.Join(p.CoworkRoot(), testtree.AcctA, testtree.Org, "local_"+testtree.Chat2, ".claude", ".claude.json"), "not json", testtree.T0)
	testtree.Write(t, filepath.Join(p.CoworkRoot(), testtree.AcctA, testtree.Org, "c2c2c2c2", ".claude", ".claude.json"), oauth("not-a-uuid", "z@z.com"), testtree.T0)

	known := identity.Known(p)

	if len(known) != 2 || known[testtree.AcctA] != "a@x.com" || known[testtree.AcctB] != "b@y.com" {
		t.Fatalf("known = %v", known)
	}
}

func TestKnownPrefersTheNewestFile(t *testing.T) {
	t.Parallel()
	p := paths.Paths{Home: t.TempDir()}
	testtree.Write(t, filepath.Join(p.CoworkRoot(), testtree.AcctA, testtree.Org, "local_"+testtree.Chat1, ".claude", ".claude.json"), oauth(testtree.AcctA, "old@x.com"), testtree.T0)
	testtree.Write(t, p.ClaudeConfig(), oauth(testtree.AcctA, "new@x.com"), testtree.T0.Add(time.Hour))
	if got := identity.Known(p)[testtree.AcctA]; got != "new@x.com" {
		t.Errorf("got %q, want new@x.com", got)
	}
}
