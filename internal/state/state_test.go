package state_test

import (
	"path/filepath"
	"testing"

	"github.com/wjames111/ccdejavu/internal/state"
	"github.com/wjames111/ccdejavu/internal/testtree"
)

func TestLoadMissingIsEmpty(t *testing.T) {
	t.Parallel()
	st, err := state.Load(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Backup != "" || !st.LastSync.IsZero() || len(st.Groups) != 0 {
		t.Errorf("got %+v, want zero state", st)
	}
}

func TestSaveThenLoad(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "nested", "state.json")
	want := state.State{
		Backup:   "/b/20260901-120000",
		LastSync: testtree.T0,
		Groups: []state.Group{
			{Root: "code", Org: testtree.Org, Accounts: []string{testtree.AcctA, testtree.AcctB}, Actions: 3},
			{Root: "cowork", Org: testtree.Org, Accounts: []string{testtree.AcctA}, Skipped: "bad file"},
		},
	}
	if err := state.Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := state.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Backup != want.Backup || !got.LastSync.Equal(want.LastSync) || len(got.Groups) != 2 {
		t.Fatalf("got %+v", got)
	}
	if got.Groups[0].Actions != 3 || got.Groups[1].Skipped != "bad file" || len(got.Groups[0].Accounts) != 2 {
		t.Errorf("groups = %+v", got.Groups)
	}
}
