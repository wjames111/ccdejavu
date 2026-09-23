package cli

import (
	"strings"
	"testing"

	"github.com/wjames111/ccdejavu/internal/paths"
	"github.com/wjames111/ccdejavu/internal/testtree"
)

func TestInstallRejectsHome(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	_, err := runCLI(t, "--home", home, "install")
	if err == nil || !strings.Contains(err.Error(), "--home") {
		t.Fatalf("err = %v, want it to mention --home", err)
	}
	if testtree.Exists(paths.Paths{Home: home}.Data()) {
		t.Error("install created ~/.ccdejavu despite the --home guard")
	}
}

func TestUninstallRejectsHome(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	_, err := runCLI(t, "--home", home, "uninstall")
	if err == nil || !strings.Contains(err.Error(), "--home") {
		t.Fatalf("err = %v, want it to mention --home", err)
	}
	if testtree.Exists(paths.Paths{Home: home}.Data()) {
		t.Error("uninstall created ~/.ccdejavu despite the --home guard")
	}
}
