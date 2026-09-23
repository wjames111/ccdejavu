package cli

import (
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
		"FAIL code org 0e0e0e0e layout",
	)
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
