package accounts_test

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/wjames111/ccdejavu/internal/accounts"
	"github.com/wjames111/ccdejavu/internal/paths"
	"github.com/wjames111/ccdejavu/internal/testtree"
)

func chat(id, title string, at time.Time) string {
	return `{"sessionId":"local_` + id + `","title":"` + title + `","lastActivityAt":` + strconv.FormatInt(at.UnixMilli(), 10) + `}`
}

func TestSummarizeCountsTitlesAndOrder(t *testing.T) {
	t.Parallel()
	p := paths.Paths{Home: t.TempDir()}
	o2 := "0e0e0e0e-0000-4000-8000-000000000002"
	c3 := "c3c3c3c3-0000-4000-8000-000000000003"
	c4 := "c4c4c4c4-0000-4000-8000-000000000004"
	c5 := "c5c5c5c5-0000-4000-8000-000000000005"
	codeA := filepath.Join(p.CodeRoot(), testtree.AcctA, testtree.Org)
	// Chat1 is the most recent chat in A's folder, but it's also in B's, so it's
	// nobody's "own" chat and must not win a Recent slot or set LastActive.
	testtree.Write(t, codeA+"/local_"+testtree.Chat1+".json", chat(testtree.Chat1, "shared", testtree.T0.Add(150*time.Minute)), testtree.T0)
	testtree.Write(t, filepath.Join(codeA, "local_"+testtree.Chat2+".json"), chat(testtree.Chat2, "middle", testtree.T0.Add(time.Hour)), testtree.T0)
	// The same chat in a second org counts once.
	testtree.Write(t, filepath.Join(p.CodeRoot(), testtree.AcctA, o2, "local_"+testtree.Chat2+".json"), chat(testtree.Chat2, "middle", testtree.T0.Add(time.Hour)), testtree.T0)
	testtree.Write(t, filepath.Join(p.CoworkRoot(), testtree.AcctA, testtree.Org, "local_"+c3+".json"), chat(c3, "newest", testtree.T0.Add(2*time.Hour)), testtree.T0)
	// No lastActivityAt: falls back to the file's mtime.
	testtree.Write(t, filepath.Join(codeA, "local_"+c4+".json"), `{"sessionId":"local_`+c4+`","title":"by mtime"}`, testtree.T0.Add(30*time.Minute))
	testtree.Write(t, filepath.Join(p.CodeRoot(), testtree.AcctB, testtree.Org, "local_"+testtree.Chat1+".json"), chat(testtree.Chat1, "b chat", testtree.T0.Add(3*time.Hour)), testtree.T0)
	// B's own chat, more recent than any of A's, so B still sorts first.
	testtree.Write(t, filepath.Join(p.CoworkRoot(), testtree.AcctB, testtree.Org, "local_"+c5+".json"), chat(c5, "b only", testtree.T0.Add(4*time.Hour)), testtree.T0)
	testtree.Write(t, filepath.Join(p.CoworkRoot(), testtree.AcctB, testtree.Org, "local_x", ".claude", ".claude.json"), `{"oauthAccount":{"accountUuid":"`+testtree.AcctB+`","emailAddress":"b@y.com"}}`, testtree.T0)
	if err := os.MkdirAll(filepath.Join(p.CodeRoot(), "not-an-account"), 0o700); err != nil {
		t.Fatal(err)
	}

	sums, err := accounts.Summarize(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(sums) != 2 || sums[0].ID != testtree.AcctB || sums[1].ID != testtree.AcctA {
		t.Fatalf("order = %+v, want B (most recent) then A", sums)
	}
	if sums[0].Email != "b@y.com" || sums[1].Email != "" {
		t.Errorf("emails = %q, %q", sums[0].Email, sums[1].Email)
	}
	b, a := sums[0], sums[1]
	if b.Chats != 2 || b.Own != 1 {
		t.Errorf("B chats = %d, own = %d, want 2 and 1", b.Chats, b.Own)
	}
	if want := []string{"b only"}; !slices.Equal(b.Recent, want) {
		t.Errorf("B recent = %q, want %q", b.Recent, want)
	}
	if a.Chats != 4 {
		t.Errorf("A chats = %d, want 4", a.Chats)
	}
	if a.Own != 3 {
		t.Errorf("A own = %d, want 3", a.Own)
	}
	if want := []string{"newest", "middle", "by mtime"}; !slices.Equal(a.Recent, want) {
		t.Errorf("A recent = %q, want %q (shared chat1 excluded even though it's the newest)", a.Recent, want)
	}
	if !a.LastActive.Equal(testtree.T0.Add(2 * time.Hour)) {
		t.Errorf("A LastActive = %v, want the newest OWN chat, not the shared one", a.LastActive)
	}
}

func TestSummarizeIdenticalAccountsHaveNoOwnChats(t *testing.T) {
	t.Parallel()
	p := paths.Paths{Home: t.TempDir()}
	for _, acct := range []string{testtree.AcctA, testtree.AcctB} {
		dir := filepath.Join(p.CodeRoot(), acct, testtree.Org)
		testtree.Write(t, filepath.Join(dir, "local_"+testtree.Chat1+".json"), chat(testtree.Chat1, "one", testtree.T0), testtree.T0)
		testtree.Write(t, filepath.Join(dir, "local_"+testtree.Chat2+".json"), chat(testtree.Chat2, "two", testtree.T0.Add(time.Hour)), testtree.T0)
	}

	sums, err := accounts.Summarize(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(sums) != 2 {
		t.Fatalf("sums = %+v, want 2 accounts", sums)
	}
	for _, s := range sums {
		if s.Chats != 2 || s.Own != 0 || len(s.Recent) != 0 || !s.LastActive.IsZero() {
			t.Errorf("account %s = %+v, want Own 0, no Recent, zero LastActive", s.ID, s)
		}
	}
}
