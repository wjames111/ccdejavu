package cli

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wjames111/ccdejavu/internal/paths"
	"github.com/wjames111/ccdejavu/internal/testtree"
)

func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

// codeHome has accounts A and B, linked, in one org under the Code tab root, and one
// chat file with the given body in account A.
func codeHome(t *testing.T, chat string) string {
	t.Helper()
	p := paths.Paths{Home: t.TempDir()}
	testtree.Mkdir(t, filepath.Join(p.CodeRoot(), testtree.AcctB, testtree.Org))
	testtree.Write(t, filepath.Join(p.CodeRoot(), testtree.AcctA, testtree.Org, "local_"+testtree.Chat1+".json"), chat, testtree.T0)
	testtree.Link(t, paths.Paths{Home: p.Home}.Links(), testtree.AcctA, testtree.AcctB)
	return p.Home
}

func expectAll(t *testing.T, out string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q:\n%s", want, out)
		}
	}
}

func TestSyncDryRunPrintsThePlan(t *testing.T) {
	t.Parallel()
	home := codeHome(t, testtree.ChatJSON(testtree.Chat1, "one"))
	out, err := runCLI(t, "--home", home, "sync", "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	expectAll(t, out, "Would back up", "copy ", "code · aaaaaaaa@example.com + bbbbbbbb@example.com: 1 planned change across 2 folders")
}

func TestSyncThenStatus(t *testing.T) {
	t.Parallel()
	home := codeHome(t, testtree.ChatJSON(testtree.Chat1, "one"))
	out, err := runCLI(t, "--home", home, "sync")
	if err != nil {
		t.Fatal(err)
	}
	expectAll(t, out, "code · aaaaaaaa@example.com + bbbbbbbb@example.com: 1 change across 2 folders")

	out, err = runCLI(t, "--home", home, "status")
	if err != nil {
		t.Fatal(err)
	}
	expectAll(t, out,
		"aaaaaaaa@example.com · org 0e0e0e0e  1 chat",
		"bbbbbbbb@example.com · org 0e0e0e0e  1 chat",
		"in sync (last sync made 1 change)",
		fmt.Sprintf("%-16s%s", "Background job:", "not installed"),
	)
	if strings.Contains(out, "never") {
		t.Errorf("status says never synced after a sync:\n%s", out)
	}
}

func TestStatusBeforeAnySync(t *testing.T) {
	t.Parallel()
	out, err := runCLI(t, "--home", t.TempDir(), "status")
	if err != nil {
		t.Fatal(err)
	}
	expectAll(t, out,
		fmt.Sprintf("%-16s%s", "Last sync:", "never"),
		fmt.Sprintf("%-16s%s", "Backup:", "none yet"),
	)
}
