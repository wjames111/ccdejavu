package cli

import (
	"bytes"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/wjames111/ccdejavu/internal/links"
	"github.com/wjames111/ccdejavu/internal/paths"
	"github.com/wjames111/ccdejavu/internal/testtree"
)

func runCLIWithInput(t *testing.T, input string, args ...string) (string, error) {
	t.Helper()
	cmd := newRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader(input))
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

// linkHome has account A (email known as a@x.com, org O) and account B (no known email, a different org).
func linkHome(t *testing.T) paths.Paths {
	t.Helper()
	p := paths.Paths{Home: t.TempDir()}
	o2 := "0e0e0e0e-0000-4000-8000-000000000002"
	at := strconv.FormatInt(testtree.T0.UnixMilli(), 10)
	testtree.Write(t, filepath.Join(p.CodeRoot(), testtree.AcctA, testtree.Org, "local_"+testtree.Chat1+".json"),
		`{"sessionId":"local_`+testtree.Chat1+`","title":"alpha","lastActivityAt":`+at+`}`, testtree.T0)
	testtree.Write(t, filepath.Join(p.CodeRoot(), testtree.AcctB, o2, "local_"+testtree.Chat2+".json"),
		`{"sessionId":"local_`+testtree.Chat2+`","title":"beta","lastActivityAt":`+at+`}`, testtree.T0)
	testtree.Write(t, filepath.Join(p.CoworkRoot(), testtree.AcctA, testtree.Org, "local_x", ".claude", ".claude.json"),
		`{"oauthAccount":{"accountUuid":"`+testtree.AcctA+`","emailAddress":"a@x.com"}}`, testtree.T0)
	return p
}

func TestLinkProposesAKnownEmail(t *testing.T) {
	t.Parallel()
	p := linkHome(t)
	out, err := runCLIWithInput(t, "y\n1\ny\n", "--home", p.Home, "link", "a@x.com", "b@y.com")
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	expectAll(t, out, "Found a@x.com", "Which account is b@y.com?", "only here:", `"beta"`, "sends that conversation")
	l, err := links.Load(p.Links())
	if err != nil {
		t.Fatal(err)
	}
	if l.EmailFor(testtree.AcctA) != "a@x.com" || l.EmailFor(testtree.AcctB) != "b@y.com" {
		t.Errorf("links = %+v", l)
	}
}

func TestLinkHidesAnAccountKnownByAnotherEmail(t *testing.T) {
	t.Parallel()
	p := linkHome(t)
	out, err := runCLIWithInput(t, "1\ny\ny\n", "--home", p.Home, "link", "c@z.com", "a@x.com")
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	first := out[:strings.Index(out, "Found a@x.com")]
	if strings.Contains(first, "aaaaaaaa") || strings.Contains(first, "2)") {
		t.Errorf("offered a@x.com's account for c@z.com:\n%s", first)
	}
	l, _ := links.Load(p.Links())
	if l.EmailFor(testtree.AcctB) != "c@z.com" {
		t.Errorf("links = %+v", l)
	}
}

func TestLinkDeclinedSavesNothing(t *testing.T) {
	t.Parallel()
	p := linkHome(t)
	out, err := runCLIWithInput(t, "y\n1\nn\n", "--home", p.Home, "link", "a@x.com", "b@y.com")
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if testtree.Exists(p.Links()) {
		t.Error("saved links after the user said no")
	}
	expectAll(t, out, "Nothing changed")
}

func TestLinkNeedsTwoAccounts(t *testing.T) {
	t.Parallel()
	p := paths.Paths{Home: t.TempDir()}
	testtree.Mkdir(t, filepath.Join(p.CodeRoot(), testtree.AcctA, testtree.Org))
	_, err := runCLIWithInput(t, "", "--home", p.Home, "link", "a@x.com", "b@y.com")
	if err == nil || !strings.Contains(err.Error(), "sign each account") {
		t.Fatalf("err = %v", err)
	}
}

func TestLinkReusesASavedEmail(t *testing.T) {
	t.Parallel()
	p := linkHome(t)
	pre := links.Links{Groups: []links.Group{{Accounts: []links.Account{
		{Email: "a@x.com", ID: testtree.AcctA},
		{Email: "b@y.com", ID: testtree.AcctB},
	}}}}
	if err := links.Save(p.Links(), pre); err != nil {
		t.Fatal(err)
	}
	out, err := runCLIWithInput(t, "y\ny\ny\n", "--home", p.Home, "link", "a@x.com", "b@y.com")
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	expectAll(t, out, "Found a@x.com", "Found b@y.com")
	l, err := links.Load(p.Links())
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Groups) != 1 || len(l.Groups[0].Accounts) != 2 {
		t.Errorf("links = %+v", l)
	}
}

func TestLinkAddsAThirdAccountWithoutOfferingLinkedOnes(t *testing.T) {
	t.Parallel()
	p := linkHome(t)
	acctC := "cccccccc-0000-4000-8000-00000000000c"
	testtree.Write(t, filepath.Join(p.CodeRoot(), acctC, testtree.Org, "local_c3c3c3c3-0000-4000-8000-000000000003.json"),
		`{"sessionId":"local_c3c3c3c3-0000-4000-8000-000000000003","title":"gamma"}`, testtree.T0)
	pre := links.Links{Groups: []links.Group{{Accounts: []links.Account{
		{Email: "a@x.com", ID: testtree.AcctA},
		{Email: "b@y.com", ID: testtree.AcctB},
	}}}}
	if err := links.Save(p.Links(), pre); err != nil {
		t.Fatal(err)
	}
	out, err := runCLIWithInput(t, "y\n1\ny\n", "--home", p.Home, "link", "b@y.com", "c@z.com")
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	i := strings.Index(out, "Which account is c@z.com?")
	if i < 0 {
		t.Fatalf("output never asked which account is c@z.com:\n%s", out)
	}
	list := out[i:]
	if strings.Contains(list, short(testtree.AcctA)) || strings.Contains(list, short(testtree.AcctB)) {
		t.Errorf("offered an already-linked account for c@z.com:\n%s", list)
	}
	l, err := links.Load(p.Links())
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Groups) != 1 || len(l.Groups[0].Accounts) != 3 {
		t.Errorf("links = %+v", l)
	}
}

func TestLinkSameEmailTwiceErrorsImmediately(t *testing.T) {
	t.Parallel()
	p := paths.Paths{Home: t.TempDir()}
	out, err := runCLIWithInput(t, "", "--home", p.Home, "link", "a@x.com", "A@x.com")
	if err == nil || !strings.Contains(err.Error(), "same") {
		t.Fatalf("err = %v\n%s", err, out)
	}
}

func TestLinkClosedInputAtUseItPromptStopsImmediately(t *testing.T) {
	t.Parallel()
	p := linkHome(t)
	out, err := runCLIWithInput(t, "", "--home", p.Home, "link", "a@x.com", "b@y.com")
	if err == nil || !strings.Contains(err.Error(), "no answer") {
		t.Fatalf("err = %v\n%s", err, out)
	}
	if strings.Contains(out, "Which account is") {
		t.Errorf("fell through to the picker list after closed input:\n%s", out)
	}
}

func TestUnlink(t *testing.T) {
	t.Parallel()
	p := linkHome(t)
	testtree.Link(t, p.Links(), testtree.AcctA, testtree.AcctB)
	out, err := runCLI(t, "--home", p.Home, "unlink", "aaaaaaaa@example.com")
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	l, _ := links.Load(p.Links())
	if len(l.Groups) != 0 {
		t.Errorf("links = %+v", l)
	}
	if _, err := runCLI(t, "--home", p.Home, "unlink", "nobody@x.com"); err == nil {
		t.Error("unlinked an email that isn't linked")
	}
}
