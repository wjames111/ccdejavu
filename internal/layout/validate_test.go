package layout_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wjames111/ccdejavu/internal/layout"
	"github.com/wjames111/ccdejavu/internal/testtree"
)

func newGroup(t *testing.T) layout.Group {
	t.Helper()
	g := layout.Group{
		Root:    layout.Root{Name: "cowork", Dir: t.TempDir(), ChatDirs: true},
		Name:    testtree.Org,
		Members: []string{layout.Member(testtree.AcctA, testtree.Org), layout.Member(testtree.AcctB, testtree.Org)},
	}
	testtree.Mkdir(t, g.Dir(layout.Member(testtree.AcctA, testtree.Org)))
	testtree.Mkdir(t, g.Dir(layout.Member(testtree.AcctB, testtree.Org)))
	return g
}

func chatPath(g layout.Group, account, id string) string {
	return filepath.Join(g.Dir(layout.Member(account, testtree.Org)), "local_"+id+".json")
}

func TestValidateAcceptsRealShapedFolders(t *testing.T) {
	t.Parallel()
	g := newGroup(t)
	testtree.Write(t, chatPath(g, testtree.AcctA, testtree.Chat1), testtree.ChatJSON(testtree.Chat1, "one"), testtree.T0)
	testtree.Write(t, chatPath(g, testtree.AcctB, testtree.Chat2), testtree.ChatJSON(testtree.Chat2, "two"), testtree.T0)
	testtree.Write(t, filepath.Join(g.Dir(layout.Member(testtree.AcctA, testtree.Org)), "cowork-gb-cache.json"), "not json", testtree.T0)
	testtree.Write(t, filepath.Join(g.Dir(layout.Member(testtree.AcctA, testtree.Org)), "deleted_"+testtree.Chat2), "1788202164972", testtree.T0)
	if err := layout.Validate(g); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRejectsNonJSONChat(t *testing.T) {
	t.Parallel()
	g := newGroup(t)
	testtree.Write(t, chatPath(g, testtree.AcctB, testtree.Chat1), "not json", testtree.T0)
	err := layout.Validate(g)
	if err == nil || !strings.Contains(err.Error(), "not a chat file") {
		t.Fatalf("got %v, want a 'not a chat file' error", err)
	}
}

func TestValidateRejectsMismatchedSessionID(t *testing.T) {
	t.Parallel()
	g := newGroup(t)
	testtree.Write(t, chatPath(g, testtree.AcctA, testtree.Chat1), testtree.ChatJSON(testtree.Chat2, "wrong"), testtree.T0)
	err := layout.Validate(g)
	if err == nil || !strings.Contains(err.Error(), "sessionId") {
		t.Fatalf("got %v, want a sessionId error", err)
	}
}

func TestCountChatsCountsOnlyChats(t *testing.T) {
	t.Parallel()
	g := newGroup(t)
	dir := g.Dir(layout.Member(testtree.AcctA, testtree.Org))
	testtree.Write(t, chatPath(g, testtree.AcctA, testtree.Chat1), testtree.ChatJSON(testtree.Chat1, "one"), testtree.T0)
	testtree.Write(t, chatPath(g, testtree.AcctA, testtree.Chat2), testtree.ChatJSON(testtree.Chat2, "two"), testtree.T0)
	testtree.Write(t, filepath.Join(dir, "deleted_"+testtree.Chat1), "1", testtree.T0)
	testtree.Write(t, filepath.Join(dir, "scheduled-tasks.json"), "{}", testtree.T0)
	n, err := layout.CountChats(dir)
	if err != nil || n != 2 {
		t.Fatalf("CountChats = %d, %v; want 2", n, err)
	}
}

func TestValidateRejectsSymlinkedChat(t *testing.T) {
	t.Parallel()
	g := newGroup(t)
	target := filepath.Join(t.TempDir(), "elsewhere.json")
	testtree.Write(t, target, testtree.ChatJSON(testtree.Chat1, "x"), testtree.T0)
	if err := os.Symlink(target, chatPath(g, testtree.AcctA, testtree.Chat1)); err != nil {
		t.Fatal(err)
	}
	err := layout.Validate(g)
	if err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("got %v, want a 'not a regular file' error", err)
	}
}
