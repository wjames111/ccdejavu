package apply_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wjames111/ccdejavu/internal/apply"
	"github.com/wjames111/ccdejavu/internal/plan"
	"github.com/wjames111/ccdejavu/internal/testtree"
)

func TestCopyKeepsBodyTimeAndMode(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "src.json")
	testtree.Write(t, src, "hello", testtree.T0)
	if err := os.Chmod(src, 0o640); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "out", "nested", "dst.json")

	if err := apply.CopyFile(src, dst); err != nil {
		t.Fatal(err)
	}
	if got := testtree.Read(t, dst); got != "hello" {
		t.Errorf("body = %q", got)
	}
	if got := testtree.MTime(t, dst); !got.Equal(testtree.T0) {
		t.Errorf("mtime = %v, want %v", got, testtree.T0)
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Errorf("mode = %v, want 0640", info.Mode().Perm())
	}
	entries, err := os.ReadDir(filepath.Dir(dst))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("temp files left behind: %v", entries)
	}
}

func TestCopyReplacesExisting(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "new.json")
	dst := filepath.Join(dir, "old.json")
	testtree.Write(t, src, "new", testtree.T0.Add(time.Hour))
	testtree.Write(t, dst, "old", testtree.T0)
	if err := apply.CopyFile(src, dst); err != nil {
		t.Fatal(err)
	}
	if got := testtree.Read(t, dst); got != "new" {
		t.Errorf("body = %q, want new", got)
	}
}

func TestRunMkdirAndTrash(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	chat := filepath.Join(dir, "app", "local_c1.json")
	folder := filepath.Join(dir, "app", "local_c1")
	testtree.Write(t, chat, "{}", testtree.T0)
	testtree.Write(t, filepath.Join(folder, "outputs", "x.txt"), "x", testtree.T0)
	made := filepath.Join(dir, "app", "made", "deep")

	n, err := apply.Run([]plan.Action{
		{Op: plan.Mkdir, Dst: made},
		{Op: plan.Trash, Src: chat, Dst: filepath.Join(dir, "trash", "run", "local_c1.json")},
		{Op: plan.Trash, Src: folder, Dst: filepath.Join(dir, "trash", "run", "local_c1")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("n = %d, want 3", n)
	}
	if !testtree.Exists(made) {
		t.Error("mkdir did not create the folder")
	}
	if testtree.Exists(chat) || testtree.Exists(folder) {
		t.Error("trashed items are still in the app folder")
	}
	if got := testtree.Read(t, filepath.Join(dir, "trash", "run", "local_c1", "outputs", "x.txt")); got != "x" {
		t.Errorf("trashed folder lost its contents: %q", got)
	}
}

func TestTrashRefusesToOverwrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	chat := filepath.Join(dir, "app", "local_c1.json")
	earlier := filepath.Join(dir, "trash", "run", "local_c1.json")
	testtree.Write(t, chat, "later", testtree.T0)
	testtree.Write(t, earlier, "earlier", testtree.T0)

	n, err := apply.Run([]plan.Action{{Op: plan.Trash, Src: chat, Dst: earlier}})

	if err == nil {
		t.Fatal("want an error when the trash path is taken")
	}
	if n != 0 {
		t.Errorf("n = %d, want 0", n)
	}
	if got := testtree.Read(t, earlier); got != "earlier" {
		t.Errorf("trashed copy was overwritten: %q", got)
	}
	if !testtree.Exists(chat) {
		t.Error("the chat left the app folder even though it wasn't trashed")
	}
}

func TestCopySkipsWhenDestinationIsNewer(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "src.json")
	dst := filepath.Join(dir, "dst.json")
	testtree.Write(t, src, "older", testtree.T0)
	testtree.Write(t, dst, "app's newer write", testtree.T0.Add(time.Hour))

	if err := apply.CopyFile(src, dst); err != nil {
		t.Fatal(err)
	}
	if got := testtree.Read(t, dst); got != "app's newer write" {
		t.Errorf("body = %q, want app's newer write", got)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("temp files left behind: %v", entries)
	}
}

func TestCopyKeepsNanosecondTimes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "src.json")
	precise := testtree.T0.Add(123456789)
	testtree.Write(t, src, "x", precise)
	dst := filepath.Join(dir, "dst.json")
	if err := apply.CopyFile(src, dst); err != nil {
		t.Fatal(err)
	}
	if got := testtree.MTime(t, dst); !got.Equal(precise) {
		t.Errorf("mtime = %v, want %v", got, precise)
	}
}

func TestFailedCopyLeavesDestinationAlone(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dst := filepath.Join(dir, "dst.json")
	testtree.Write(t, dst, "old", testtree.T0)
	if err := apply.CopyFile(dir, dst); err == nil {
		t.Fatal("copying a folder as a file should fail")
	}
	if got := testtree.Read(t, dst); got != "old" {
		t.Errorf("destination changed to %q", got)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("temp files left behind: %v", entries)
	}
}

func TestRunStopsAtFirstFailure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	later := filepath.Join(dir, "later")
	n, err := apply.Run([]plan.Action{
		{Op: plan.Copy, Src: filepath.Join(dir, "missing"), Dst: filepath.Join(dir, "dst")},
		{Op: plan.Mkdir, Dst: later},
	})
	if err == nil {
		t.Fatal("want an error for a missing source")
	}
	if n != 0 {
		t.Errorf("n = %d, want 0", n)
	}
	if testtree.Exists(later) {
		t.Error("ran an action after a failure")
	}
}
