package links_test

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/wjames111/ccdejavu/internal/links"
)

const (
	idA = "aaaaaaaa-0000-4000-8000-00000000000a"
	idB = "bbbbbbbb-0000-4000-8000-00000000000b"
	idC = "cccccccc-0000-4000-8000-00000000000c"
	idD = "dddddddd-0000-4000-8000-00000000000d"
)

func acct(email, id string) links.Account { return links.Account{Email: email, ID: id} }

func emails(g links.Group) string {
	var out []string
	for _, a := range g.Accounts {
		out = append(out, a.Email)
	}
	return strings.Join(out, ",")
}

func TestLinkNewPairThenAddThenMerge(t *testing.T) {
	t.Parallel()
	var l links.Links
	if err := l.Link(acct("A@x.com", idA), acct("b@y.com", idB)); err != nil {
		t.Fatal(err)
	}
	if err := l.Link(acct("c@z.com", idC), acct("a@x.com", idA)); err != nil {
		t.Fatal(err)
	}
	if len(l.Groups) != 1 || emails(l.Groups[0]) != "a@x.com,b@y.com,c@z.com" {
		t.Fatalf("after add: %+v", l.Groups)
	}
	var other links.Links
	if err := other.Link(acct("a@x.com", idA), acct("b@y.com", idB)); err != nil {
		t.Fatal(err)
	}
	if err := other.Link(acct("c@z.com", idC), acct("d@w.com", idD)); err != nil {
		t.Fatal(err)
	}
	if err := other.Link(acct("b@y.com", idB), acct("d@w.com", idD)); err != nil {
		t.Fatal(err)
	}
	if len(other.Groups) != 1 || len(other.Groups[0].Accounts) != 4 {
		t.Fatalf("after merge: %+v", other.Groups)
	}
}

func TestLinkAlreadyLinkedIsANoOp(t *testing.T) {
	t.Parallel()
	var l links.Links
	for range 2 {
		if err := l.Link(acct("a@x.com", idA), acct("b@y.com", idB)); err != nil {
			t.Fatal(err)
		}
	}
	if len(l.Groups) != 1 || len(l.Groups[0].Accounts) != 2 {
		t.Fatalf("got %+v", l.Groups)
	}
}

func TestLinkRejectsConflicts(t *testing.T) {
	t.Parallel()
	var l links.Links
	if err := l.Link(acct("a@x.com", idA), acct("b@y.com", idA)); err == nil {
		t.Error("linked an account to itself")
	}
	if err := l.Link(acct("a@x.com", idA), acct("b@y.com", idB)); err != nil {
		t.Fatal(err)
	}
	if err := l.Link(acct("a@x.com", idC), acct("d@w.com", idD)); err == nil || !strings.Contains(err.Error(), "a@x.com") {
		t.Errorf("email tied to two accounts: err = %v", err)
	}
	if err := l.Link(acct("z@x.com", idA), acct("d@w.com", idD)); err == nil || !strings.Contains(err.Error(), "already linked as a@x.com") {
		t.Errorf("account with two emails: err = %v", err)
	}
}

func TestUnlinkDropsGroupsOfOne(t *testing.T) {
	t.Parallel()
	var l links.Links
	if err := l.Link(acct("a@x.com", idA), acct("b@y.com", idB)); err != nil {
		t.Fatal(err)
	}
	if l.Unlink("nobody@x.com") {
		t.Error("unlinked an email that isn't linked")
	}
	if !l.Unlink("B@y.com") {
		t.Fatal("didn't unlink b@y.com")
	}
	if len(l.Groups) != 0 {
		t.Errorf("group of one kept: %+v", l.Groups)
	}
}

func TestEmailFor(t *testing.T) {
	t.Parallel()
	var l links.Links
	if err := l.Link(acct("a@x.com", idA), acct("b@y.com", idB)); err != nil {
		t.Fatal(err)
	}
	if got := l.EmailFor(idB); got != "b@y.com" {
		t.Errorf("EmailFor = %q", got)
	}
	if got := l.EmailFor(idC); got != "" {
		t.Errorf("EmailFor unknown = %q", got)
	}
}

func TestSets(t *testing.T) {
	t.Parallel()
	var l links.Links
	if err := l.Link(acct("a@x.com", idA), acct("b@y.com", idB)); err != nil {
		t.Fatal(err)
	}
	sets := l.Sets()
	if len(sets) != 1 || sets[0].Name != "a@x.com + b@y.com" {
		t.Fatalf("got %+v", sets)
	}
	if !slices.Contains(sets[0].Accounts, idA) || !slices.Contains(sets[0].Accounts, idB) {
		t.Fatalf("accounts = %v, want both ids", sets[0].Accounts)
	}
}

func TestSaveThenLoad(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "nested", "links.json")
	empty, err := links.Load(path)
	if err != nil || len(empty.Groups) != 0 {
		t.Fatalf("missing file: %+v, %v", empty, err)
	}
	var l links.Links
	if err := l.Link(acct("a@x.com", idA), acct("b@y.com", idB)); err != nil {
		t.Fatal(err)
	}
	if err := links.Save(path, l); err != nil {
		t.Fatal(err)
	}
	got, err := links.Load(path)
	if err != nil || len(got.Groups) != 1 || emails(got.Groups[0]) != "a@x.com,b@y.com" {
		t.Fatalf("got %+v, %v", got, err)
	}
}
