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

// snapshot maps chat id -> member -> what that member has.
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
		marker := first(g.Members, have)
		for _, m := range g.Members {
			if _, ok := have[m]; !ok {
				actions = append(actions, Action{Op: Copy, Src: marker, Dst: filepath.Join(g.Dir(m), "deleted_"+id)})
			}
			if f, ok := s.chats[id][m]; ok {
				actions = append(actions, Action{Op: Trash, Src: f.path, Dst: trashPath(g, trashRoot, m, f.path)})
			}
			if d, ok := s.chatDirs[id][m]; ok {
				actions = append(actions, Action{Op: Trash, Src: d, Dst: trashPath(g, trashRoot, m, d)})
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
		actions = append(actions, newestWins(g, have, func(m string) string {
			return filepath.Join(g.Dir(m), name)
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
	for _, m := range g.Members {
		dir := g.Dir(m)
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
				put(s.chats, id, m, file{path: path, mtime: info.ModTime()})
			case layout.ChatDir:
				put(s.chatDirs, id, m, path)
			case layout.Marker:
				if !e.Type().IsRegular() {
					s.blocked[id] = true
					continue
				}
				put(s.markers, id, m, path)
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

// planChatDir merges one Cowork chat folder across member folders, file by file.
// Files are never removed from a chat folder here; only a delete marker does that.
// A folder holding anything unexpected is left alone rather than half-synced.
func planChatDir(g layout.Group, id string, have map[string]string) ([]Action, error) {
	files := map[string]map[string]file{} // relative path -> member -> file
	dirs := map[string]map[string]bool{}  // relative path -> member -> present
	for m, root := range have {
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
				put(dirs, rel, m, true)
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
				put(files, rel, m, file{path: path, mtime: info.ModTime()})
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
		for _, m := range g.Members {
			if !present[m] {
				out = append(out, Action{Op: Mkdir, Dst: filepath.Join(g.Dir(m), name, rel)})
			}
		}
	}
	for rel, copies := range files {
		out = append(out, newestWins(g, copies, func(m string) string {
			return filepath.Join(g.Dir(m), name, rel)
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
func newestWins(g layout.Group, have map[string]file, dst func(member string) string) []Action {
	var best file
	found := false
	for _, m := range g.Members {
		f, ok := have[m]
		if ok && (!found || f.mtime.After(best.mtime)) {
			best, found = f, true
		}
	}
	if !found {
		return nil
	}
	var out []Action
	for _, m := range g.Members {
		if f, ok := have[m]; ok && !f.mtime.Before(best.mtime) {
			continue
		}
		out = append(out, Action{Op: Copy, Src: best.path, Dst: dst(m)})
	}
	return out
}

// trashPath mirrors src's place under the app folder inside this run's trash.
func trashPath(g layout.Group, trashRoot, member, src string) string {
	return filepath.Join(trashRoot, filepath.Base(g.Root.Dir), member, filepath.Base(src))
}

func put[V any](m map[string]map[string]V, key, member string, v V) {
	if m[key] == nil {
		m[key] = map[string]V{}
	}
	m[key][member] = v
}

// first returns the value for the first member, in sorted order, that has one.
func first[V any](members []string, have map[string]V) V {
	for _, m := range members {
		if v, ok := have[m]; ok {
			return v
		}
	}
	var zero V
	return zero
}
