package layout_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/wjames111/ccdejavu/internal/layout"
	"github.com/wjames111/ccdejavu/internal/paths"
	"github.com/wjames111/ccdejavu/internal/testtree"
)

func TestRoots(t *testing.T) {
	t.Parallel()
	p := paths.Paths{Home: "/h"}
	roots := layout.Roots(p)
	if len(roots) != 2 {
		t.Fatalf("got %d roots, want 2", len(roots))
	}
	if roots[0].Name != "code" || roots[0].Dir != p.CodeRoot() || roots[0].ChatDirs {
		t.Errorf("code root: %+v", roots[0])
	}
	if roots[1].Name != "cowork" || roots[1].Dir != p.CoworkRoot() || !roots[1].ChatDirs {
		t.Errorf("cowork root: %+v", roots[1])
	}
}

func TestClassify(t *testing.T) {
	t.Parallel()
	id := testtree.Chat1
	cases := []struct {
		name     string
		isDir    bool
		chatDirs bool
		kind     layout.Kind
		id       string
	}{
		{"local_" + id + ".json", false, false, layout.Chat, id},
		{"deleted_" + id, false, false, layout.Marker, id},
		{"local_" + id, true, true, layout.ChatDir, id},
		{"local_" + id, true, false, layout.Other, ""},
		{"local_" + id + ".json", true, true, layout.Other, ""},
		{"scheduled-tasks.json", false, true, layout.Other, ""},
		{"cowork-policy-limits-cache.json", false, true, layout.Other, ""},
		{"rpm", true, true, layout.Other, ""},
		{"c1c1c1c1", true, true, layout.Other, ""},
		{"local_C1C1C1C1-0000-4000-8000-000000000001.json", false, false, layout.Other, ""},
		{".ccdejavu-123.tmp", false, true, layout.Other, ""},
	}
	for _, c := range cases {
		kind, got := layout.Classify(c.name, c.isDir, c.chatDirs)
		if kind != c.kind || got != c.id {
			t.Errorf("Classify(%q, dir=%v, chatDirs=%v) = %v %q, want %v %q",
				c.name, c.isDir, c.chatDirs, kind, got, c.kind, c.id)
		}
	}
}

func TestDiscoverGroupsAccountsByOrg(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	root := layout.Root{Name: "code", Dir: dir}
	lonely := "0e0e0e0e-0000-4000-8000-000000000002"
	testtree.Mkdir(t, filepath.Join(dir, testtree.AcctA, testtree.Org))
	testtree.Mkdir(t, filepath.Join(dir, testtree.AcctB, testtree.Org))
	testtree.Mkdir(t, filepath.Join(dir, testtree.AcctA, lonely))
	testtree.Mkdir(t, filepath.Join(dir, "skills-plugin"))
	testtree.Write(t, filepath.Join(dir, ".DS_Store"), "", testtree.T0)

	groups, err := layout.Discover([]layout.Root{root})
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 {
		t.Fatalf("got %d groups, want 2: %+v", len(groups), groups)
	}
	both, one := groups[0], groups[1]
	wantBoth := []string{layout.Member(testtree.AcctA, testtree.Org), layout.Member(testtree.AcctB, testtree.Org)}
	if both.Name != testtree.Org || !slices.Equal(both.Members, wantBoth) || !both.Syncable() {
		t.Errorf("shared org group: %+v", both)
	}
	wantOne := []string{layout.Member(testtree.AcctA, lonely)}
	if one.Name != lonely || !slices.Equal(one.Members, wantOne) || one.Syncable() {
		t.Errorf("single-account group: %+v", one)
	}
	if got, want := both.Dir(layout.Member(testtree.AcctB, testtree.Org)), filepath.Join(dir, testtree.AcctB, testtree.Org); got != want {
		t.Errorf("Dir = %q, want %q", got, want)
	}
}

func TestGroupsHoldEveryOrgFolderOfLinkedAccounts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	o2 := "0e0e0e0e-0000-4000-8000-000000000002"
	acctC := "cccccccc-0000-4000-8000-00000000000c"
	acctD := "dddddddd-0000-4000-8000-00000000000d"
	for _, m := range []string{
		layout.Member(testtree.AcctA, testtree.Org),
		layout.Member(testtree.AcctA, o2),
		layout.Member(testtree.AcctB, testtree.Org),
		layout.Member(acctC, o2),
		layout.Member(acctD, testtree.Org),
	} {
		testtree.Mkdir(t, filepath.Join(dir, m))
	}
	root := layout.Root{Name: "code", Dir: dir}
	groups, err := layout.Groups([]layout.Root{root}, []layout.Set{
		{Name: "ab", Accounts: []string{testtree.AcctA, testtree.AcctB}},
		{Name: "c", Accounts: []string{acctC}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 {
		t.Fatalf("got %d groups, want 2: %+v", len(groups), groups)
	}
	want := []string{
		layout.Member(testtree.AcctA, testtree.Org),
		layout.Member(testtree.AcctA, o2),
		layout.Member(testtree.AcctB, testtree.Org),
	}
	if groups[0].Name != "ab" || !slices.Equal(groups[0].Members, want) || !groups[0].Syncable() {
		t.Errorf("linked group = %+v, want members %v", groups[0], want)
	}
	if groups[1].Name != "c" || len(groups[1].Members) != 1 || groups[1].Syncable() {
		t.Errorf("single-folder group = %+v", groups[1])
	}
}

func TestAccountFoldersMissingRootIsEmpty(t *testing.T) {
	t.Parallel()
	folders, err := layout.AccountFolders(layout.Root{Name: "code", Dir: filepath.Join(t.TempDir(), "missing")})
	if err != nil || len(folders) != 0 {
		t.Fatalf("got %v, %v", folders, err)
	}
}

func TestDiscoverMissingRootIsEmpty(t *testing.T) {
	t.Parallel()
	root := layout.Root{Name: "code", Dir: filepath.Join(t.TempDir(), "missing")}
	groups, err := layout.Discover([]layout.Root{root})
	if err != nil || len(groups) != 0 {
		t.Fatalf("got %v, %v; want no groups and no error", groups, err)
	}
}

func TestDiscoverIgnoresSymlinksAndUppercase(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	real := filepath.Join(t.TempDir(), "real")
	testtree.Mkdir(t, filepath.Join(real, testtree.Org))
	testtree.Mkdir(t, filepath.Join(dir, testtree.AcctA))
	if err := os.Symlink(real, filepath.Join(dir, testtree.AcctB)); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(real, testtree.Org), filepath.Join(dir, testtree.AcctA, testtree.Org)); err != nil {
		t.Fatal(err)
	}
	testtree.Mkdir(t, filepath.Join(dir, "CCCCCCCC-0000-4000-8000-00000000000C", testtree.Org))

	groups, err := layout.Discover([]layout.Root{{Name: "code", Dir: dir}})
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 0 {
		t.Fatalf("got %+v, want symlinked and uppercase folders ignored", groups)
	}
}
