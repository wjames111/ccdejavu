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
	codeA := filepath.Join(p.CodeRoot(), testtree.AcctA, testtree.Org)
	testtree.Write(t, filepath.Join(codeA, "local_"+testtree.Chat1+".json"), chat(testtree.Chat1, "oldest", testtree.T0), testtree.T0)
	testtree.Write(t, filepath.Join(codeA, "local_"+testtree.Chat2+".json"), chat(testtree.Chat2, "middle", testtree.T0.Add(time.Hour)), testtree.T0)
	// The same chat in a second org counts once.
	testtree.Write(t, filepath.Join(p.CodeRoot(), testtree.AcctA, o2, "local_"+testtree.Chat2+".json"), chat(testtree.Chat2, "middle", testtree.T0.Add(time.Hour)), testtree.T0)
	testtree.Write(t, filepath.Join(p.CoworkRoot(), testtree.AcctA, testtree.Org, "local_"+c3+".json"), chat(c3, "newest", testtree.T0.Add(2*time.Hour)), testtree.T0)
	// No lastActivityAt: falls back to the file's mtime.
	testtree.Write(t, filepath.Join(codeA, "local_"+c4+".json"), `{"sessionId":"local_`+c4+`","title":"by mtime"}`, testtree.T0.Add(30*time.Minute))
	testtree.Write(t, filepath.Join(p.CodeRoot(), testtree.AcctB, testtree.Org, "local_"+testtree.Chat1+".json"), chat(testtree.Chat1, "b chat", testtree.T0.Add(3*time.Hour)), testtree.T0)
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
	a := sums[1]
	if a.Chats != 4 {
		t.Errorf("A chats = %d, want 4", a.Chats)
	}
	if want := []string{"newest", "middle", "by mtime"}; !slices.Equal(a.Recent, want) {
		t.Errorf("A recent = %q, want %q", a.Recent, want)
	}
}
