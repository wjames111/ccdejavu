package plan_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/wjames111/ccdejavu/internal/layout"
	"github.com/wjames111/ccdejavu/internal/plan"
	"github.com/wjames111/ccdejavu/internal/testtree"
)

// fixture is a two-account group plus a replacer that shortens paths and
// ids so expected plans read like "copy APP/A/O/local_c1.json -> ...".
type fixture struct {
	g     layout.Group
	trash string
	short *strings.Replacer
}

func setup(t *testing.T, chatDirs bool) fixture {
	t.Helper()
	base := t.TempDir()
	app := filepath.Join(base, "app", "local-agent-mode-sessions")
	trash := filepath.Join(base, "trash")
	g := layout.Group{
		Root:    layout.Root{Name: "cowork", Dir: app, ChatDirs: chatDirs},
		Name:    testtree.Org,
		Members: []string{layout.Member(testtree.AcctA, testtree.Org), layout.Member(testtree.AcctB, testtree.Org)},
	}
	testtree.Mkdir(t, g.Dir(layout.Member(testtree.AcctA, testtree.Org)))
	testtree.Mkdir(t, g.Dir(layout.Member(testtree.AcctB, testtree.Org)))
	short := strings.NewReplacer(
		app, "APP", trash, "TRASH",
		testtree.AcctA, "A", testtree.AcctB, "B", testtree.Org, "O",
		testtree.Chat1, "c1", testtree.Chat2, "c2",
	)
	return fixture{g: g, trash: trash, short: short}
}

func (f fixture) in(account string, parts ...string) string {
	return filepath.Join(append([]string{f.g.Dir(layout.Member(account, testtree.Org))}, parts...)...)
}

func (f fixture) expect(t *testing.T, want ...string) {
	t.Helper()
	actions, err := plan.Build(f.g, f.trash)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(actions))
	for i, a := range actions {
		got[i] = f.short.Replace(a.String())
	}
	if !slices.Equal(got, want) {
		t.Fatalf("plan mismatch\n got: %q\nwant: %q", got, want)
	}
}

func chatFile(id string) string { return "local_" + id + ".json" }

func TestCopiesMissingChat(t *testing.T) {
	t.Parallel()
	f := setup(t, true)
	testtree.Write(t, f.in(testtree.AcctA, chatFile(testtree.Chat1)), testtree.ChatJSON(testtree.Chat1, "one"), testtree.T0)
	f.expect(t, "copy APP/A/O/local_c1.json -> APP/B/O/local_c1.json")
}

func TestCopiesBothDirections(t *testing.T) {
	t.Parallel()
	f := setup(t, true)
	testtree.Write(t, f.in(testtree.AcctA, chatFile(testtree.Chat1)), testtree.ChatJSON(testtree.Chat1, "one"), testtree.T0)
	testtree.Write(t, f.in(testtree.AcctB, chatFile(testtree.Chat2)), testtree.ChatJSON(testtree.Chat2, "two"), testtree.T0)
	f.expect(t,
		"copy APP/B/O/local_c2.json -> APP/A/O/local_c2.json",
		"copy APP/A/O/local_c1.json -> APP/B/O/local_c1.json",
	)
}

func TestNewestCopyWins(t *testing.T) {
	t.Parallel()
	f := setup(t, true)
	testtree.Write(t, f.in(testtree.AcctA, chatFile(testtree.Chat1)), testtree.ChatJSON(testtree.Chat1, "old"), testtree.T0)
	testtree.Write(t, f.in(testtree.AcctB, chatFile(testtree.Chat1)), testtree.ChatJSON(testtree.Chat1, "new"), testtree.T0.Add(time.Hour))
	f.expect(t, "copy APP/B/O/local_c1.json -> APP/A/O/local_c1.json")
}

func TestEqualTimesPlanNothing(t *testing.T) {
	t.Parallel()
	f := setup(t, true)
	testtree.Write(t, f.in(testtree.AcctA, chatFile(testtree.Chat1)), testtree.ChatJSON(testtree.Chat1, "a"), testtree.T0)
	testtree.Write(t, f.in(testtree.AcctB, chatFile(testtree.Chat1)), testtree.ChatJSON(testtree.Chat1, "b"), testtree.T0)
	f.expect(t)
}

func TestDeleteMarkerSpreadsAndTrashesChat(t *testing.T) {
	t.Parallel()
	f := setup(t, true)
	testtree.Write(t, f.in(testtree.AcctA, "deleted_"+testtree.Chat1), "1788202164972", testtree.T0)
	testtree.Write(t, f.in(testtree.AcctB, chatFile(testtree.Chat1)), testtree.ChatJSON(testtree.Chat1, "gone"), testtree.T0)
	testtree.Write(t, f.in(testtree.AcctB, "local_"+testtree.Chat1, "outputs", "x.txt"), "x", testtree.T0)
	f.expect(t,
		"copy APP/A/O/deleted_c1 -> APP/B/O/deleted_c1",
		"trash APP/B/O/local_c1 -> TRASH/local-agent-mode-sessions/B/O/local_c1",
		"trash APP/B/O/local_c1.json -> TRASH/local-agent-mode-sessions/B/O/local_c1.json",
	)
}

func TestChatFolderMergesFileByFile(t *testing.T) {
	t.Parallel()
	f := setup(t, true)
	for _, a := range []string{testtree.AcctA, testtree.AcctB} {
		testtree.Write(t, f.in(a, chatFile(testtree.Chat1)), testtree.ChatJSON(testtree.Chat1, "same"), testtree.T0)
	}
	dir := "local_" + testtree.Chat1
	testtree.Write(t, f.in(testtree.AcctA, dir, "outputs", "report.md"), "old", testtree.T0)
	testtree.Write(t, f.in(testtree.AcctB, dir, "outputs", "report.md"), "new", testtree.T0.Add(time.Hour))
	testtree.Write(t, f.in(testtree.AcctB, dir, "uploads", "photo.png"), "png", testtree.T0)
	f.expect(t,
		"copy APP/B/O/local_c1/outputs/report.md -> APP/A/O/local_c1/outputs/report.md",
		"mkdir APP/A/O/local_c1/uploads",
		"copy APP/B/O/local_c1/uploads/photo.png -> APP/A/O/local_c1/uploads/photo.png",
	)
}

func TestUnknownEntriesAreIgnored(t *testing.T) {
	t.Parallel()
	f := setup(t, true)
	for _, name := range []string{
		"scheduled-tasks.json",
		"cowork-policy-limits-cache.json",
		"cowork-gb-cache.json",
		".DS_Store",
		filepath.Join("rpm", "manifest.json"),
		filepath.Join("c1c1c1c1", "uploads-tmp", "x"),
	} {
		testtree.Write(t, f.in(testtree.AcctA, name), "{}", testtree.T0)
	}
	f.expect(t)
}

func TestCodeRootIgnoresChatFolders(t *testing.T) {
	t.Parallel()
	f := setup(t, false)
	testtree.Write(t, f.in(testtree.AcctA, "local_"+testtree.Chat1, "outputs", "x.txt"), "x", testtree.T0)
	f.expect(t)
}

func TestSymlinkedMarkersAndChatsAreIgnored(t *testing.T) {
	t.Parallel()
	f := setup(t, true)
	target := filepath.Join(t.TempDir(), "elsewhere")
	testtree.Write(t, target, testtree.ChatJSON(testtree.Chat1, "x"), testtree.T0)
	for _, name := range []string{"deleted_" + testtree.Chat1, chatFile(testtree.Chat2), "local_" + testtree.Chat2} {
		if err := os.Symlink(target, f.in(testtree.AcctA, name)); err != nil {
			t.Fatal(err)
		}
	}
	f.expect(t)
}

func TestSymlinkWhereAChatFolderBelongsBlocksThatChat(t *testing.T) {
	t.Parallel()
	f := setup(t, true)
	if err := os.Symlink(t.TempDir(), f.in(testtree.AcctA, "local_"+testtree.Chat1)); err != nil {
		t.Fatal(err)
	}
	testtree.Write(t, f.in(testtree.AcctB, chatFile(testtree.Chat1)), testtree.ChatJSON(testtree.Chat1, "one"), testtree.T0)
	testtree.Write(t, f.in(testtree.AcctB, "local_"+testtree.Chat1, "outputs", "x.txt"), "x", testtree.T0)
	f.expect(t)
}

func TestSymlinkInsideAChatFolderBlocksThatChat(t *testing.T) {
	t.Parallel()
	f := setup(t, true)
	for _, a := range []string{testtree.AcctA, testtree.AcctB} {
		testtree.Write(t, f.in(a, chatFile(testtree.Chat1)), testtree.ChatJSON(testtree.Chat1, "same"), testtree.T0)
	}
	testtree.Write(t, f.in(testtree.AcctB, "local_"+testtree.Chat1, "outputs", "x.txt"), "x", testtree.T0)
	testtree.Mkdir(t, f.in(testtree.AcctA, "local_"+testtree.Chat1))
	if err := os.Symlink(t.TempDir(), f.in(testtree.AcctA, "local_"+testtree.Chat1, "outputs")); err != nil {
		t.Fatal(err)
	}
	f.expect(t)
}

func TestFileWhereAFolderBelongsBlocksThatChat(t *testing.T) {
	t.Parallel()
	f := setup(t, true)
	for _, a := range []string{testtree.AcctA, testtree.AcctB} {
		testtree.Write(t, f.in(a, chatFile(testtree.Chat1)), testtree.ChatJSON(testtree.Chat1, "same"), testtree.T0)
	}
	testtree.Write(t, f.in(testtree.AcctA, "local_"+testtree.Chat1, "uploads"), "a file", testtree.T0)
	testtree.Write(t, f.in(testtree.AcctB, "local_"+testtree.Chat1, "uploads", "photo.png"), "png", testtree.T0)
	f.expect(t)
}

func TestFolderWhereAChatFileBelongsBlocksThatChat(t *testing.T) {
	t.Parallel()
	f := setup(t, true)
	testtree.Mkdir(t, f.in(testtree.AcctA, chatFile(testtree.Chat1)))
	testtree.Write(t, f.in(testtree.AcctB, chatFile(testtree.Chat1)), testtree.ChatJSON(testtree.Chat1, "one"), testtree.T0)
	f.expect(t)
}

func TestLeftoverTempFilesAreNotSpread(t *testing.T) {
	t.Parallel()
	f := setup(t, true)
	for _, a := range []string{testtree.AcctA, testtree.AcctB} {
		testtree.Write(t, f.in(a, chatFile(testtree.Chat1)), testtree.ChatJSON(testtree.Chat1, "same"), testtree.T0)
	}
	testtree.Write(t, f.in(testtree.AcctA, "local_"+testtree.Chat1, ".ccdejavu-123.tmp"), "half", testtree.T0)
	testtree.Mkdir(t, f.in(testtree.AcctB, "local_"+testtree.Chat1))
	f.expect(t)
}

func TestMarkerTrashesTheSameAccountsChat(t *testing.T) {
	t.Parallel()
	f := setup(t, true)
	testtree.Write(t, f.in(testtree.AcctA, "deleted_"+testtree.Chat1), "1788202164972", testtree.T0)
	testtree.Write(t, f.in(testtree.AcctA, chatFile(testtree.Chat1)), testtree.ChatJSON(testtree.Chat1, "gone"), testtree.T0)
	f.expect(t,
		"copy APP/A/O/deleted_c1 -> APP/B/O/deleted_c1",
		"trash APP/A/O/local_c1.json -> TRASH/local-agent-mode-sessions/A/O/local_c1.json",
	)
}

func TestAccountFilesInsideChatFoldersStayPut(t *testing.T) {
	t.Parallel()
	f := setup(t, true)
	for _, a := range []string{testtree.AcctA, testtree.AcctB} {
		testtree.Write(t, f.in(a, chatFile(testtree.Chat1)), testtree.ChatJSON(testtree.Chat1, "same"), testtree.T0)
	}
	dir := "local_" + testtree.Chat1
	for _, name := range []string{
		filepath.Join(".claude", ".claude.json"),
		filepath.Join(".claude", "policy-limits.json"),
		filepath.Join(".claude", "cache", "x"),
		filepath.Join(".claude", "backups", ".claude.json.backup.1"),
	} {
		testtree.Write(t, f.in(testtree.AcctA, dir, name), "{}", testtree.T0)
	}
	testtree.Write(t, f.in(testtree.AcctA, dir, ".claude", "projects", "p.jsonl"), "{}", testtree.T0)
	testtree.Write(t, f.in(testtree.AcctA, dir, "uploads-tmp", "x"), "up", testtree.T0)
	testtree.Mkdir(t, f.in(testtree.AcctB, dir))
	f.expect(t,
		"mkdir APP/B/O/local_c1/.claude",
		"mkdir APP/B/O/local_c1/.claude/projects",
		"copy APP/A/O/local_c1/.claude/projects/p.jsonl -> APP/B/O/local_c1/.claude/projects/p.jsonl",
	)
}

func TestMissingChatFolderIsCreatedWithEmptySubfolders(t *testing.T) {
	t.Parallel()
	f := setup(t, true)
	for _, a := range []string{testtree.AcctA, testtree.AcctB} {
		testtree.Write(t, f.in(a, chatFile(testtree.Chat1)), testtree.ChatJSON(testtree.Chat1, "same"), testtree.T0)
	}
	testtree.Write(t, f.in(testtree.AcctA, "local_"+testtree.Chat1, "outputs", "r.md"), "r", testtree.T0)
	testtree.Mkdir(t, f.in(testtree.AcctA, "local_"+testtree.Chat1, "empty"))
	f.expect(t,
		"mkdir APP/B/O/local_c1",
		"mkdir APP/B/O/local_c1/empty",
		"mkdir APP/B/O/local_c1/outputs",
		"copy APP/A/O/local_c1/outputs/r.md -> APP/B/O/local_c1/outputs/r.md",
	)
}
