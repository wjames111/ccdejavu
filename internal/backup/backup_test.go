package backup_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wjames111/ccdejavu/internal/backup"
	"github.com/wjames111/ccdejavu/internal/layout"
	"github.com/wjames111/ccdejavu/internal/testtree"
)

func roots(t *testing.T) (layout.Root, layout.Root) {
	t.Helper()
	app := filepath.Join(t.TempDir(), "Claude")
	return layout.Root{Name: "code", Dir: filepath.Join(app, "claude-code-sessions")},
		layout.Root{Name: "cowork", Dir: filepath.Join(app, "local-agent-mode-sessions"), ChatDirs: true}
}

func TestBackupCopiesEveryRootInFull(t *testing.T) {
	t.Parallel()
	code, cowork := roots(t)
	org := func(r layout.Root, parts ...string) string {
		return filepath.Join(append([]string{r.Dir, testtree.AcctA, testtree.Org}, parts...)...)
	}
	testtree.Write(t, org(code, "local_"+testtree.Chat1+".json"), testtree.ChatJSON(testtree.Chat1, "one"), testtree.T0)
	testtree.Write(t, org(cowork, "cowork-gb-cache.json"), "{}", testtree.T0)
	testtree.Write(t, org(cowork, "local_"+testtree.Chat2, "outputs", "x.txt"), "x", testtree.T0)
	dest := filepath.Join(t.TempDir(), "backup")

	if err := backup.Run([]layout.Root{code, cowork}, dest); err != nil {
		t.Fatal(err)
	}

	in := func(root string, parts ...string) string {
		return filepath.Join(append([]string{dest, root, testtree.AcctA, testtree.Org}, parts...)...)
	}
	chat := in("claude-code-sessions", "local_"+testtree.Chat1+".json")
	if got := testtree.Read(t, chat); got != testtree.ChatJSON(testtree.Chat1, "one") {
		t.Errorf("chat body = %q", got)
	}
	if !testtree.MTime(t, chat).Equal(testtree.T0) {
		t.Error("backup changed the chat's mtime")
	}
	if !testtree.Exists(in("local-agent-mode-sessions", "cowork-gb-cache.json")) {
		t.Error("backup skipped a cache file; it should copy everything")
	}
	if got := testtree.Read(t, in("local-agent-mode-sessions", "local_"+testtree.Chat2, "outputs", "x.txt")); got != "x" {
		t.Errorf("nested file = %q", got)
	}
}

func TestBackupKeepsFolderAndFileModes(t *testing.T) {
	t.Parallel()
	code, _ := roots(t)
	folder := filepath.Join(code.Dir, testtree.AcctA, "tasks")
	file := filepath.Join(folder, "x.json")
	testtree.Write(t, file, "{}", testtree.T0)
	if err := os.Chmod(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(folder, testtree.T0, testtree.T0); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "backup")
	if err := backup.Run([]layout.Root{code}, dest); err != nil {
		t.Fatal(err)
	}
	gotFolder, err := os.Stat(filepath.Join(dest, "claude-code-sessions", testtree.AcctA, "tasks"))
	if err != nil {
		t.Fatal(err)
	}
	if gotFolder.Mode().Perm() != 0o755 || !gotFolder.ModTime().Equal(testtree.T0) {
		t.Errorf("folder mode %v mtime %v, want 0755 and %v", gotFolder.Mode().Perm(), gotFolder.ModTime(), testtree.T0)
	}
	gotFile, err := os.Stat(filepath.Join(dest, "claude-code-sessions", testtree.AcctA, "tasks", "x.json"))
	if err != nil {
		t.Fatal(err)
	}
	if gotFile.Mode().Perm() != 0o600 {
		t.Errorf("file mode %v, want 0600", gotFile.Mode().Perm())
	}
}

func TestBackupSkipsMissingRoots(t *testing.T) {
	t.Parallel()
	code, _ := roots(t)
	dest := filepath.Join(t.TempDir(), "backup")
	if err := backup.Run([]layout.Root{code}, dest); err != nil {
		t.Fatal(err)
	}
	if testtree.Exists(filepath.Join(dest, "claude-code-sessions")) {
		t.Error("backed up a root that doesn't exist")
	}
}

func TestBackupKeepsSymlinks(t *testing.T) {
	t.Parallel()
	code, _ := roots(t)
	testtree.Mkdir(t, filepath.Join(code.Dir, testtree.AcctA))
	if err := os.Symlink("somewhere", filepath.Join(code.Dir, testtree.AcctA, "link")); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "backup")
	if err := backup.Run([]layout.Root{code}, dest); err != nil {
		t.Fatal(err)
	}
	got, err := os.Readlink(filepath.Join(dest, "claude-code-sessions", testtree.AcctA, "link"))
	if err != nil || got != "somewhere" {
		t.Fatalf("Readlink = %q, %v; want somewhere", got, err)
	}
}
