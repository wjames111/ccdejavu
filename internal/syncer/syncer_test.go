package syncer_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wjames111/ccdejavu/internal/paths"
	"github.com/wjames111/ccdejavu/internal/state"
	"github.com/wjames111/ccdejavu/internal/syncer"
	"github.com/wjames111/ccdejavu/internal/testtree"
)

// newHome builds a home folder where both roots have accounts A and B in one org.
func newHome(t *testing.T) paths.Paths {
	t.Helper()
	p := paths.Paths{Home: t.TempDir()}
	for _, root := range []string{p.CodeRoot(), p.CoworkRoot()} {
		testtree.Mkdir(t, filepath.Join(root, testtree.AcctA, testtree.Org))
		testtree.Mkdir(t, filepath.Join(root, testtree.AcctB, testtree.Org))
	}
	testtree.Link(t, p.Links(), testtree.AcctA, testtree.AcctB)
	return p
}

func orgFile(root, account, name string) string {
	return filepath.Join(root, account, testtree.Org, name)
}

func chatName(id string) string { return "local_" + id + ".json" }

func run(t *testing.T, p paths.Paths, dryRun bool) (state.State, string) {
	t.Helper()
	var out bytes.Buffer
	st, err := syncer.Run(syncer.Options{
		Paths:  p,
		DryRun: dryRun,
		Out:    &out,
		Now:    func() time.Time { return testtree.T0 },
	})
	if err != nil {
		t.Fatal(err)
	}
	return st, out.String()
}

func TestFirstRunBacksUpThenSyncsBothRoots(t *testing.T) {
	t.Parallel()
	p := newHome(t)
	testtree.Write(t, orgFile(p.CodeRoot(), testtree.AcctA, chatName(testtree.Chat1)), testtree.ChatJSON(testtree.Chat1, "code"), testtree.T0)
	testtree.Write(t, orgFile(p.CoworkRoot(), testtree.AcctA, chatName(testtree.Chat2)), testtree.ChatJSON(testtree.Chat2, "cowork"), testtree.T0)

	st, out := run(t, p, false)

	if !testtree.Exists(orgFile(p.CodeRoot(), testtree.AcctB, chatName(testtree.Chat1))) {
		t.Error("Code tab chat did not reach account B")
	}
	if !testtree.Exists(orgFile(p.CoworkRoot(), testtree.AcctB, chatName(testtree.Chat2))) {
		t.Error("Cowork chat did not reach account B")
	}
	wantBackup := filepath.Join(p.Backups(), "20260901-120000")
	if st.Backup != wantBackup {
		t.Errorf("Backup = %q, want %q", st.Backup, wantBackup)
	}
	backedUp := filepath.Join(wantBackup, "claude-code-sessions", testtree.AcctA, testtree.Org, chatName(testtree.Chat1))
	if !testtree.Exists(backedUp) {
		t.Error("backup is missing account A's chat")
	}
	notYet := filepath.Join(wantBackup, "claude-code-sessions", testtree.AcctB, testtree.Org, chatName(testtree.Chat1))
	if testtree.Exists(notYet) {
		t.Error("backup was taken after the sync, not before")
	}
	if !strings.Contains(out, "Backed up") {
		t.Errorf("output = %q, want a backup line", out)
	}
	if st.LastSync.IsZero() {
		t.Error("LastSync not recorded")
	}
}

func TestSecondRunChangesNothing(t *testing.T) {
	t.Parallel()
	p := newHome(t)
	testtree.Write(t, orgFile(p.CodeRoot(), testtree.AcctA, chatName(testtree.Chat1)), testtree.ChatJSON(testtree.Chat1, "one"), testtree.T0)
	run(t, p, false)

	st, _ := run(t, p, false)

	for _, g := range st.Groups {
		if g.Actions != 0 || g.Skipped != "" {
			t.Errorf("second run: %+v, want no changes", g)
		}
	}
	backups, err := os.ReadDir(p.Backups())
	if err != nil || len(backups) != 1 {
		t.Errorf("backups = %v, %v; want exactly one", backups, err)
	}
}

func TestDryRunChangesNothing(t *testing.T) {
	t.Parallel()
	p := newHome(t)
	testtree.Write(t, orgFile(p.CodeRoot(), testtree.AcctA, chatName(testtree.Chat1)), testtree.ChatJSON(testtree.Chat1, "one"), testtree.T0)

	st, out := run(t, p, true)

	if testtree.Exists(orgFile(p.CodeRoot(), testtree.AcctB, chatName(testtree.Chat1))) {
		t.Error("dry run copied a chat")
	}
	// p.Data() itself already exists (newHome's links.json lives there); dry run must still
	// take none of a real sync's actions.
	if testtree.Exists(p.State()) {
		t.Error("dry run recorded state")
	}
	if testtree.Exists(p.Backups()) {
		t.Error("dry run took a backup")
	}
	if !strings.Contains(out, "Would back up") || !strings.Contains(out, "copy ") {
		t.Errorf("output = %q, want the backup notice and the planned copy", out)
	}
	if st.Groups[0].Actions != 1 {
		t.Errorf("planned actions = %d, want 1", st.Groups[0].Actions)
	}
}

func TestInvalidGroupIsSkippedOthersStillSync(t *testing.T) {
	t.Parallel()
	p := newHome(t)
	testtree.Write(t, orgFile(p.CodeRoot(), testtree.AcctA, chatName(testtree.Chat1)), "not json", testtree.T0)
	testtree.Write(t, orgFile(p.CoworkRoot(), testtree.AcctA, chatName(testtree.Chat2)), testtree.ChatJSON(testtree.Chat2, "ok"), testtree.T0)

	st, _ := run(t, p, false)

	if testtree.Exists(orgFile(p.CodeRoot(), testtree.AcctB, chatName(testtree.Chat1))) {
		t.Error("a bad Code tab folder was synced anyway")
	}
	if !testtree.Exists(orgFile(p.CoworkRoot(), testtree.AcctB, chatName(testtree.Chat2))) {
		t.Error("the good Cowork folder was not synced")
	}
	if st.Groups[0].Root != "code" || !strings.Contains(st.Groups[0].Skipped, "not a chat file") {
		t.Errorf("code group = %+v, want skipped with a reason", st.Groups[0])
	}
}

func TestBackupFailureStopsSync(t *testing.T) {
	t.Parallel()
	p := newHome(t)
	testtree.Write(t, orgFile(p.CodeRoot(), testtree.AcctA, chatName(testtree.Chat1)), testtree.ChatJSON(testtree.Chat1, "one"), testtree.T0)
	testtree.Write(t, p.Backups(), "a file where the backups folder should be", testtree.T0)

	_, err := syncer.Run(syncer.Options{Paths: p, Out: &bytes.Buffer{}, Now: func() time.Time { return testtree.T0 }})

	if err == nil || !strings.Contains(err.Error(), "nothing was synced") {
		t.Fatalf("err = %v, want a backup failure", err)
	}
	if testtree.Exists(orgFile(p.CodeRoot(), testtree.AcctB, chatName(testtree.Chat1))) {
		t.Error("synced even though the backup failed")
	}
}

func TestBackupFailureCleansUpPartialFolder(t *testing.T) {
	t.Parallel()
	p := newHome(t)
	file := orgFile(p.CodeRoot(), testtree.AcctA, chatName(testtree.Chat1))
	testtree.Write(t, file, testtree.ChatJSON(testtree.Chat1, "one"), testtree.T0)
	if err := os.Chmod(file, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(file, 0o600) })

	_, err := syncer.Run(syncer.Options{Paths: p, Out: &bytes.Buffer{}, Now: func() time.Time { return testtree.T0 }})

	if err == nil || !strings.Contains(err.Error(), "nothing was synced") {
		t.Fatalf("err = %v, want a backup failure", err)
	}
	entries, err := os.ReadDir(p.Backups())
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".partial") {
			t.Errorf("partial backup folder left behind: %v", entries)
		}
	}
}

func TestRunWaitsForTheLock(t *testing.T) {
	t.Parallel()
	p := newHome(t)
	unlock, err := syncer.Lock(p.Lock())
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := syncer.Run(syncer.Options{Paths: p, Out: io.Discard, Now: func() time.Time { return testtree.T0 }})
		done <- err
	}()
	select {
	case <-done:
		t.Fatal("Run finished while another sync held the lock")
	case <-time.After(100 * time.Millisecond):
	}
	unlock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run never finished after the lock was released")
	}
}

func TestPartialApplyIsReported(t *testing.T) {
	t.Parallel()
	p := newHome(t)
	testtree.Write(t, orgFile(p.CodeRoot(), testtree.AcctA, "deleted_"+testtree.Chat1), "1788202164972", testtree.T0)
	testtree.Write(t, orgFile(p.CodeRoot(), testtree.AcctB, chatName(testtree.Chat1)), testtree.ChatJSON(testtree.Chat1, "gone"), testtree.T0)
	taken := filepath.Join(p.Trash(), "20260901-120000", "claude-code-sessions", testtree.AcctB, testtree.Org, chatName(testtree.Chat1))
	testtree.Write(t, taken, "already in the trash", testtree.T0)

	st, _ := run(t, p, false)

	code := st.Groups[0]
	if code.Root != "code" || !strings.Contains(code.Skipped, "stopped after 0 of 2 changes") || code.Actions != 0 {
		t.Errorf("code group = %+v, want stopped after 0 of 2 with 0 actions", code)
	}
	if got := testtree.Read(t, taken); got != "already in the trash" {
		t.Errorf("trash was overwritten: %q", got)
	}
}

func TestNoLinksMeansNoSyncAndNoBackup(t *testing.T) {
	t.Parallel()
	p := paths.Paths{Home: t.TempDir()}
	testtree.Write(t, orgFile(p.CodeRoot(), testtree.AcctA, chatName(testtree.Chat1)), testtree.ChatJSON(testtree.Chat1, "one"), testtree.T0)
	testtree.Mkdir(t, filepath.Join(p.CodeRoot(), testtree.AcctB, testtree.Org))

	st, _ := run(t, p, false)

	if len(st.Groups) != 0 || st.Backup != "" {
		t.Errorf("state = %+v, want no groups and no backup", st)
	}
	if testtree.Exists(orgFile(p.CodeRoot(), testtree.AcctB, chatName(testtree.Chat1))) {
		t.Error("synced accounts that aren't linked")
	}
}

func TestLinkedAccountsInDifferentOrgsSync(t *testing.T) {
	t.Parallel()
	p := paths.Paths{Home: t.TempDir()}
	o2 := "0e0e0e0e-0000-4000-8000-000000000002"
	testtree.Write(t, orgFile(p.CodeRoot(), testtree.AcctA, chatName(testtree.Chat1)), testtree.ChatJSON(testtree.Chat1, "work"), testtree.T0)
	testtree.Mkdir(t, filepath.Join(p.CodeRoot(), testtree.AcctB, o2))
	testtree.Link(t, p.Links(), testtree.AcctA, testtree.AcctB)

	run(t, p, false)

	if !testtree.Exists(filepath.Join(p.CodeRoot(), testtree.AcctB, o2, chatName(testtree.Chat1))) {
		t.Error("chat didn't reach the linked account's other org")
	}
}

func TestBacksUpAgainWhenAnAccountJoinsAnExistingGroup(t *testing.T) {
	t.Parallel()
	p := newHome(t)
	testtree.Write(t, orgFile(p.CodeRoot(), testtree.AcctA, chatName(testtree.Chat1)), testtree.ChatJSON(testtree.Chat1, "one"), testtree.T0)
	run(t, p, false)
	first, err := os.ReadDir(p.Backups())
	if err != nil || len(first) != 1 {
		t.Fatalf("first backup = %v, %v; want exactly one", first, err)
	}

	acctC := "cccccccc-0000-4000-8000-00000000000c"
	testtree.Write(t, orgFile(p.CodeRoot(), acctC, chatName(testtree.Chat1)), testtree.ChatJSON(testtree.Chat1, "c's copy"), testtree.T0)
	testtree.Link(t, p.Links(), testtree.AcctA, testtree.AcctB, acctC)

	st, err := syncer.Run(syncer.Options{Paths: p, Out: &bytes.Buffer{}, Now: func() time.Time { return testtree.T0.Add(time.Hour) }})
	if err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadDir(p.Backups())
	if err != nil || len(second) != 2 {
		t.Fatalf("backups after linking C = %v, %v; want two", second, err)
	}
	wantBackup := filepath.Join(p.Backups(), "20260901-130000")
	if st.Backup != wantBackup {
		t.Errorf("Backup = %q, want %q", st.Backup, wantBackup)
	}

	if _, err := syncer.Run(syncer.Options{Paths: p, Out: &bytes.Buffer{}, Now: func() time.Time { return testtree.T0.Add(2 * time.Hour) }}); err != nil {
		t.Fatal(err)
	}
	third, err := os.ReadDir(p.Backups())
	if err != nil || len(third) != 2 {
		t.Errorf("backups after an unchanged group synced again = %v, %v; want still two", third, err)
	}
}

func TestUnlinkingDoesNotTriggerAnotherBackup(t *testing.T) {
	t.Parallel()
	p := paths.Paths{Home: t.TempDir()}
	acctC := "cccccccc-0000-4000-8000-00000000000c"
	testtree.Write(t, orgFile(p.CodeRoot(), testtree.AcctA, chatName(testtree.Chat1)), testtree.ChatJSON(testtree.Chat1, "one"), testtree.T0)
	testtree.Mkdir(t, filepath.Join(p.CodeRoot(), testtree.AcctB, testtree.Org))
	testtree.Mkdir(t, filepath.Join(p.CodeRoot(), acctC, testtree.Org))
	testtree.Link(t, p.Links(), testtree.AcctA, testtree.AcctB, acctC)
	run(t, p, false)
	first, err := os.ReadDir(p.Backups())
	if err != nil || len(first) != 1 {
		t.Fatalf("first backup = %v, %v; want exactly one", first, err)
	}

	testtree.Link(t, p.Links(), testtree.AcctA, testtree.AcctB) // relink, dropping C

	st, err := syncer.Run(syncer.Options{Paths: p, Out: &bytes.Buffer{}, Now: func() time.Time { return testtree.T0.Add(time.Hour) }})
	if err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadDir(p.Backups())
	if err != nil || len(second) != 1 {
		t.Errorf("backups after unlinking C = %v, %v; want still one", second, err)
	}
	wantBackup := filepath.Join(p.Backups(), "20260901-120000")
	if st.Backup != wantBackup {
		t.Errorf("Backup = %q, want %q (no new backup should have been taken)", st.Backup, wantBackup)
	}
}

func TestUpgradingFromAnOldStateFileBacksUpAgain(t *testing.T) {
	t.Parallel()
	p := paths.Paths{Home: t.TempDir()}
	testtree.Write(t, orgFile(p.CodeRoot(), testtree.AcctA, chatName(testtree.Chat1)), testtree.ChatJSON(testtree.Chat1, "one"), testtree.T0)
	testtree.Mkdir(t, filepath.Join(p.CodeRoot(), testtree.AcctB, testtree.Org))
	testtree.Link(t, p.Links(), testtree.AcctA, testtree.AcctB)
	old := `{"backup":"/old/backup","lastSync":"2026-09-01T12:00:00Z","groups":[{"root":"code","org":"` +
		testtree.Org + `","accounts":["` + testtree.AcctA + `","` + testtree.AcctB + `"],"actions":0}]}`
	testtree.Write(t, p.State(), old, testtree.T0)

	st, _ := run(t, p, false)

	entries, err := os.ReadDir(p.Backups())
	if err != nil || len(entries) != 1 {
		t.Fatalf("backups = %v, %v; want exactly one new backup", entries, err)
	}
	want := filepath.Join(p.Backups(), entries[0].Name())
	if st.Backup != want {
		t.Errorf("Backup = %q, want %q", st.Backup, want)
	}
}

func TestLeavesOtherFilesAlone(t *testing.T) {
	t.Parallel()
	p := newHome(t)
	cache := orgFile(p.CoworkRoot(), testtree.AcctA, "cowork-policy-limits-cache.json")
	tasksA := orgFile(p.CoworkRoot(), testtree.AcctA, "scheduled-tasks.json")
	tasksB := orgFile(p.CoworkRoot(), testtree.AcctB, "scheduled-tasks.json")
	testtree.Write(t, cache, `{"limit":"reached"}`, testtree.T0)
	testtree.Write(t, tasksA, `["a"]`, testtree.T0.Add(time.Hour))
	testtree.Write(t, tasksB, `["b"]`, testtree.T0)

	run(t, p, false)

	if testtree.Exists(orgFile(p.CoworkRoot(), testtree.AcctB, "cowork-policy-limits-cache.json")) {
		t.Error("an account cache was copied to another account")
	}
	if got := testtree.Read(t, tasksB); got != `["b"]` {
		t.Errorf("account B's scheduled tasks changed to %q", got)
	}
	if !testtree.MTime(t, tasksB).Equal(testtree.T0) {
		t.Error("account B's scheduled tasks were touched")
	}
}
