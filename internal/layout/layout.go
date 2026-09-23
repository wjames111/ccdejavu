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

// Group is a set of chat folders under one root that are kept matched.
type Group struct {
	Root    Root
	Name    string   // what messages call the group
	Members []string // folders as "account/org" paths under Root.Dir, sorted
}

// Member names one account's org folder inside a root.
func Member(account, org string) string { return account + "/" + org }

// Dir is the absolute path of one member folder.
func (g Group) Dir(member string) string { return filepath.Join(g.Root.Dir, member) }

// Syncable reports whether there is more than one folder to sync between.
func (g Group) Syncable() bool { return len(g.Members) > 1 }

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
		folders, err := AccountFolders(r)
		if err != nil {
			return nil, err
		}
		byOrg := map[string][]string{}
		for a, orgs := range folders {
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
			members := make([]string, len(accts))
			for i, a := range accts {
				members[i] = Member(a, o)
			}
			groups = append(groups, Group{Root: r, Name: o, Members: members})
		}
	}
	return groups, nil
}

// AccountFolders maps each account under a root to its org folders. A missing root yields nothing.
func AccountFolders(r Root) (map[string][]string, error) {
	accounts, err := uuidDirs(r.Dir)
	if err != nil {
		return nil, err
	}
	out := map[string][]string{}
	for _, a := range accounts {
		orgs, err := uuidDirs(filepath.Join(r.Dir, a))
		if err != nil {
			return nil, err
		}
		out[a] = orgs
	}
	return out, nil
}

// Set is a named list of accounts whose chats are kept matched.
type Set struct {
	Name     string
	Accounts []string
}

// Groups builds one group per root per set, holding every org folder of every account in the set.
func Groups(roots []Root, sets []Set) ([]Group, error) {
	var groups []Group
	for _, r := range roots {
		folders, err := AccountFolders(r)
		if err != nil {
			return nil, err
		}
		for _, s := range sets {
			var members []string
			for _, a := range s.Accounts {
				for _, o := range folders[a] {
					members = append(members, Member(a, o))
				}
			}
			if len(members) == 0 {
				continue
			}
			sort.Strings(members)
			groups = append(groups, Group{Root: r, Name: s.Name, Members: members})
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
