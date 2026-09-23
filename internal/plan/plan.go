// Package plan works out what a sync would change, without changing anything.
package plan

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/wjames111/ccdejavu/internal/layout"
)

// Op is one kind of change.
type Op int

const (
	Copy  Op = iota // copy Src over Dst, keeping Src's mtime and permissions
	Mkdir           // create Dst
	Trash           // move Src to Dst inside this run's trash folder
)

func (o Op) String() string {
	switch o {
	case Copy:
		return "copy"
	case Mkdir:
		return "mkdir"
	case Trash:
		return "trash"
	}
	return fmt.Sprintf("op(%d)", int(o))
}

// Action is one change to make.
type Action struct {
	Op  Op
	Src string
	Dst string
}

func (a Action) String() string {
	if a.Op == Mkdir {
		return "mkdir " + a.Dst
	}
	return fmt.Sprintf("%s %s -> %s", a.Op, a.Src, a.Dst)
}

type file struct {
	path  string
	mtime time.Time
}

// snapshot maps chat id -> account -> what that account has.
type snapshot struct {
	chats    map[string]map[string]file
	chatDirs map[string]map[string]string
	markers  map[string]map[string]string
	blocked  map[string]bool // chat ids whose entries look unexpected in some account
}

// Build plans one group's sync. trashRoot is this run's trash folder. The
// result is sorted by destination so plans are stable and easy to read.
func Build(g layout.Group, trashRoot string) ([]Action, error) {
	s, err := scan(g)
	if err != nil {
		return nil, err
	}

	var actions []Action
	for id, have := range s.markers {
		if s.blocked[id] {
			continue
		}
		marker := first(g.Accounts, have)
		for _, a := range g.Accounts {
			if _, ok := have[a]; !ok {
				actions = append(actions, Action{Op: Copy, Src: marker, Dst: filepath.Join(g.OrgDir(a), "deleted_"+id)})
			}
			if f, ok := s.chats[id][a]; ok {
				actions = append(actions, Action{Op: Trash, Src: f.path, Dst: trashPath(g, trashRoot, a, f.path)})
			}
			if d, ok := s.chatDirs[id][a]; ok {
				actions = append(actions, Action{Op: Trash, Src: d, Dst: trashPath(g, trashRoot, a, d)})
			}
		}
	}

	for id, have := range s.chats {
		if s.blocked[id] {
			continue
		}
		if _, deleted := s.markers[id]; deleted {
			continue
		}
		name := "local_" + id + ".json"
		actions = append(actions, newestWins(g, have, func(a string) string {
			return filepath.Join(g.OrgDir(a), name)
		})...)
	}

	for id, have := range s.chatDirs {
		if s.blocked[id] {
			continue
		}
		if _, deleted := s.markers[id]; deleted {
			continue
		}
		dirActions, err := planChatDir(g, id, have)
		if err != nil {
			return nil, err
		}
		actions = append(actions, dirActions...)
	}

	sort.Slice(actions, func(i, j int) bool {
		if actions[i].Dst != actions[j].Dst {
			return actions[i].Dst < actions[j].Dst
		}
		return actions[i].Op < actions[j].Op
	})
	return actions, nil
}

func scan(g layout.Group) (snapshot, error) {
	s := snapshot{
		chats:    map[string]map[string]file{},
		chatDirs: map[string]map[string]string{},
		markers:  map[string]map[string]string{},
		blocked:  map[string]bool{},
	}
	for _, a := range g.Accounts {
		dir := g.OrgDir(a)
		entries, err := os.ReadDir(dir)
		if err != nil {
			return s, err
		}
		for _, e := range entries {
			kind, id := layout.Classify(e.Name(), e.IsDir(), g.Root.ChatDirs)
			path := filepath.Join(dir, e.Name())
			switch kind {
			case layout.Chat:
				info, err := e.Info()
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				if err != nil {
					return s, err
				}
				if !info.Mode().IsRegular() {
					s.blocked[id] = true
					continue
				}
				put(s.chats, id, a, file{path: path, mtime: info.ModTime()})
			case layout.ChatDir:
				put(s.chatDirs, id, a, path)
			case layout.Marker:
				if !e.Type().IsRegular() {
					s.blocked[id] = true
					continue
				}
				put(s.markers, id, a, path)
			case layout.Other:
				// A chat's name on the wrong kind of entry (a symlink, or a file where a folder belongs) would send writes astray.
				if k, odd := layout.Classify(e.Name(), !e.IsDir(), g.Root.ChatDirs); k != layout.Other {
					s.blocked[odd] = true
				}
			}
		}
	}
	return s, nil
}

// errUnexpected stops a chat folder walk that found something other than files and folders.
var errUnexpected = errors.New("unexpected entry in chat folder")

// planChatDir merges one Cowork chat folder across accounts, file by file.
// Files are never removed from a chat folder here; only a delete marker does that.
// A folder holding anything unexpected is left alone rather than half-synced.
func planChatDir(g layout.Group, id string, have map[string]string) ([]Action, error) {
	files := map[string]map[string]file{} // relative path -> account -> file
	dirs := map[string]map[string]bool{}  // relative path -> account -> present
	for a, root := range have {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			if notChatData[rel] {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			switch {
			case d.IsDir():
				put(dirs, rel, a, true)
			case d.Type().IsRegular():
				if isTemp(d.Name()) {
					return nil
				}
				info, err := d.Info()
				if errors.Is(err, fs.ErrNotExist) {
					return nil
				}
				if err != nil {
					return err
				}
				put(files, rel, a, file{path: path, mtime: info.ModTime()})
			default:
				return errUnexpected
			}
			return nil
		})
		if errors.Is(err, errUnexpected) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
	}
	for rel := range files {
		if _, clash := dirs[rel]; clash {
			return nil, nil
		}
	}

	name := "local_" + id
	var out []Action
	for rel, present := range dirs {
		for _, a := range g.Accounts {
			if !present[a] {
				out = append(out, Action{Op: Mkdir, Dst: filepath.Join(g.OrgDir(a), name, rel)})
			}
		}
	}
	for rel, copies := range files {
		out = append(out, newestWins(g, copies, func(a string) string {
			return filepath.Join(g.OrgDir(a), name, rel)
		})...)
	}
	return out, nil
}

// notChatData lists paths inside a chat folder that aren't the chat itself:
// the signed-in account's settings and scratch uploads.
var notChatData = map[string]bool{
	filepath.Join(".claude", ".claude.json"):       true,
	filepath.Join(".claude", "policy-limits.json"): true,
	filepath.Join(".claude", "cache"):              true,
	filepath.Join(".claude", "backups"):            true,
	"uploads-tmp":                                  true,
}

// isTemp matches the temp files apply.CopyFile writes, in case a run was killed mid-copy.
func isTemp(name string) bool {
	return strings.HasPrefix(name, ".ccdejavu-") && strings.HasSuffix(name, ".tmp")
}

// newestWins copies the newest copy over every older or missing one. Equal
// times are left alone: with one account signed in at a time, equal means same.
func newestWins(g layout.Group, have map[string]file, dst func(account string) string) []Action {
	var best file
	found := false
	for _, a := range g.Accounts {
		f, ok := have[a]
		if ok && (!found || f.mtime.After(best.mtime)) {
			best, found = f, true
		}
	}
	if !found {
		return nil
	}
	var out []Action
	for _, a := range g.Accounts {
		if f, ok := have[a]; ok && !f.mtime.Before(best.mtime) {
			continue
		}
		out = append(out, Action{Op: Copy, Src: best.path, Dst: dst(a)})
	}
	return out
}

// trashPath mirrors src's place under the app folder inside this run's trash.
func trashPath(g layout.Group, trashRoot, account, src string) string {
	return filepath.Join(trashRoot, filepath.Base(g.Root.Dir), account, g.Org, filepath.Base(src))
}

func put[V any](m map[string]map[string]V, key, account string, v V) {
	if m[key] == nil {
		m[key] = map[string]V{}
	}
	m[key][account] = v
}

// first returns the value for the first account, in sorted order, that has one.
func first[V any](accounts []string, have map[string]V) V {
	for _, a := range accounts {
		if v, ok := have[a]; ok {
			return v
		}
	}
	var zero V
	return zero
}
