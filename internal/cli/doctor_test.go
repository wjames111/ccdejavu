package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wjames111/ccdejavu/internal/launchd"
	"github.com/wjames111/ccdejavu/internal/paths"
	"github.com/wjames111/ccdejavu/internal/testtree"
)

func TestDoctorFailsOnAnEmptyHome(t *testing.T) {
	t.Parallel()
	out, err := runCLI(t, "--home", t.TempDir(), "doctor")
	if err == nil {
		t.Fatal("doctor passed on a home with no app folder")
	}
	expectAll(t, out,
		"FAIL Claude desktop app folder",
		"FAIL chat folders",
		"FAIL background job: not installed",
	)
}

func TestDoctorReportsABadLayout(t *testing.T) {
	t.Parallel()
	home := codeHome(t, "not json")
	out, err := runCLI(t, "--home", home, "doctor")
	if err == nil {
		t.Fatal("doctor passed with a broken chat file")
	}
	expectAll(t, out,
		"ok   Claude desktop app folder",
		"ok   chat folders",
		"ok   linked accounts",
		"FAIL code · aaaaaaaa@example.com + bbbbbbbb@example.com layout",
	)
}

func TestDoctorFailsLinkedAccountsCheckWithoutLinks(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	p := paths.Paths{Home: home}
	testtree.Mkdir(t, filepath.Join(p.CodeRoot(), testtree.AcctA, testtree.Org))
	testtree.Mkdir(t, filepath.Join(p.CodeRoot(), testtree.AcctB, testtree.Org))

	out, err := runCLI(t, "--home", home, "doctor")
	if err == nil {
		t.Fatal("doctor passed with accounts but no links")
	}
	expectAll(t, out, "FAIL linked accounts: none yet; run: ccdejavu link <email> <email>")
}

func TestDoctorFailsWhenLinkedGroupsCantBeBuilt(t *testing.T) {
	t.Parallel()
	home := codeHome(t, testtree.ChatJSON(testtree.Chat1, "one"))
	p := paths.Paths{Home: home}
	acctDir := filepath.Join(p.CodeRoot(), testtree.AcctA)
	if err := os.Chmod(acctDir, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(acctDir, 0o700) })

	out, err := runCLI(t, "--home", home, "doctor")
	if err == nil {
		t.Fatal("doctor passed with unreadable account folders")
	}
	expectAll(t, out, "FAIL linked groups")
}

func TestDoctorReportsAMissingBinary(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	p := paths.Paths{Home: home}
	testtree.Mkdir(t, p.LaunchAgents())
	plist := "<plist><dict><key>ProgramArguments</key><array><string>/nonexistent/ccdejavu</string></array></dict></plist>"
	testtree.Write(t, launchd.DefinitionPath(p), plist, testtree.T0)

	out, err := runCLI(t, "--home", home, "doctor")
	if err == nil {
		t.Fatal("doctor passed with a background job binary that no longer exists")
	}
	expectAll(t, out, "FAIL background job: it runs /nonexistent/ccdejavu, which no longer exists")
}
