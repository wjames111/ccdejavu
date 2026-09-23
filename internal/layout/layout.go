// Package layout finds the desktop app's per-account chat folders and checks
// they still look the way ccdejavu expects.
package layout

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/wjames111/ccdejavu/internal/paths"
)

// Root is one of the app's per-account index folders.
type Root struct {
	Name string // "code" or "cowork", for messages and state
	Dir  string
	// ChatDirs is true when a chat can also own a folder named local_<uuid>/.
	ChatDirs bool
}

// Roots lists the two index folders ccdejavu syncs.
func Roots(p paths.Paths) []Root {
	return []Root{
		{Name: "code", Dir: p.CodeRoot()},
		{Name: "cowork", Dir: p.CoworkRoot(), ChatDirs: true},
	}
}

// Group is one org's folder under one root, across every account that has it.
type Group struct {
	Root     Root
	Org      string
	Accounts []string // sorted
}

// OrgDir is the folder the app reads for this group when signed in as account.
func (g Group) OrgDir(account string) string {
	return filepath.Join(g.Root.Dir, account, g.Org)
}

// Syncable reports whether there is more than one account to sync between.
func (g Group) Syncable() bool { return len(g.Accounts) > 1 }

const uuidPattern = `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`

var (
	uuidRe    = regexp.MustCompile(`^` + uuidPattern + `$`)
	chatRe    = regexp.MustCompile(`^local_(` + uuidPattern + `)\.json$`)
	chatDirRe = regexp.MustCompile(`^local_(` + uuidPattern + `)$`)
	markerRe  = regexp.MustCompile(`^deleted_(` + uuidPattern + `)$`)
)

// IsUUID reports whether s is a lowercase hyphenated UUID, the only form the app uses.
func IsUUID(s string) bool { return uuidRe.MatchString(s) }

// Kind says what an entry in an org folder is.
type Kind int

const (
	Other Kind = iota
	Chat
	ChatDir
	Marker
)

// Classify names an org folder entry and returns its chat id. Anything not
// on the allow-list is Other and is never read, written, or removed.
func Classify(name string, isDir, chatDirs bool) (Kind, string) {
	if isDir {
		if m := chatDirRe.FindStringSubmatch(name); chatDirs && m != nil {
			return ChatDir, m[1]
		}
		return Other, ""
	}
	if m := chatRe.FindStringSubmatch(name); m != nil {
		return Chat, m[1]
	}
	if m := markerRe.FindStringSubmatch(name); m != nil {
		return Marker, m[1]
	}
	return Other, ""
}

// Discover lists every (root, org) group, sorted by org within each root.
// A root that doesn't exist yields nothing.
func Discover(roots []Root) ([]Group, error) {
	var groups []Group
	for _, r := range roots {
		accounts, err := uuidDirs(r.Dir)
		if err != nil {
			return nil, err
		}
		byOrg := map[string][]string{}
		for _, a := range accounts {
			orgs, err := uuidDirs(filepath.Join(r.Dir, a))
			if err != nil {
				return nil, err
			}
			for _, o := range orgs {
				byOrg[o] = append(byOrg[o], a)
			}
		}
		orgs := make([]string, 0, len(byOrg))
		for o := range byOrg {
			orgs = append(orgs, o)
		}
		sort.Strings(orgs)
		for _, o := range orgs {
			accts := byOrg[o]
			sort.Strings(accts)
			groups = append(groups, Group{Root: r, Org: o, Accounts: accts})
		}
	}
	return groups, nil
}

// uuidDirs lists the real (non-symlink) UUID-named folders in dir.
func uuidDirs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() && IsUUID(e.Name()) {
			out = append(out, e.Name())
		}
	}
	return out, nil
}
