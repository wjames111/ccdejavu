// Package accounts summarizes each account found on this Mac, to help match accounts to emails.
package accounts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/wjames111/ccdejavu/internal/identity"
	"github.com/wjames111/ccdejavu/internal/layout"
	"github.com/wjames111/ccdejavu/internal/paths"
)

// Summary describes one account's chats across both roots and all its orgs.
type Summary struct {
	ID         string
	Email      string   // known email, or "" if none was found on this Mac
	Chats      int      // distinct chats
	Own        int      // chats that exist only in this account
	Recent     []string // up to 3 most recent chat titles, among this account's own chats
	LastActive time.Time
}

type chat struct {
	title string
	at    time.Time
}

// Summarize lists every account in either root, most recently active first.
func Summarize(p paths.Paths) ([]Summary, error) {
	known := identity.Known(p)
	byAccount := map[string]map[string]chat{}
	for _, r := range layout.Roots(p) {
		folders, err := layout.AccountFolders(r)
		if err != nil {
			return nil, err
		}
		for account, orgs := range folders {
			if byAccount[account] == nil {
				byAccount[account] = map[string]chat{}
			}
			for _, org := range orgs {
				if err := collect(filepath.Join(r.Dir, account, org), byAccount[account]); err != nil {
					return nil, err
				}
			}
		}
	}
	// A chat id seen in more than one account isn't "own" to any of them: v0.1.0
	// already synced it, so it can't help tell two already-synced accounts apart.
	seenIn := map[string]int{}
	for _, chats := range byAccount {
		for id := range chats {
			seenIn[id]++
		}
	}

	sums := make([]Summary, 0, len(byAccount))
	for account, chats := range byAccount {
		var own []chat
		for id, c := range chats {
			if seenIn[id] == 1 {
				own = append(own, c)
			}
		}
		sort.Slice(own, func(i, j int) bool { return own[i].at.After(own[j].at) })
		s := Summary{ID: account, Email: known[account], Chats: len(chats), Own: len(own)}
		if len(own) > 0 {
			s.LastActive = own[0].at
		}
		for _, c := range own {
			if len(s.Recent) == 3 {
				break
			}
			if c.title != "" {
				s.Recent = append(s.Recent, c.title)
			}
		}
		sums = append(sums, s)
	}
	sort.Slice(sums, func(i, j int) bool {
		if !sums[i].LastActive.Equal(sums[j].LastActive) {
			return sums[i].LastActive.After(sums[j].LastActive)
		}
		return sums[i].ID < sums[j].ID
	})
	return sums, nil
}

// collect adds one org folder's chats, keeping the newest copy of each chat.
func collect(dir string, into map[string]chat) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		kind, id := layout.Classify(e.Name(), e.IsDir(), false)
		if kind != layout.Chat {
			continue
		}
		c, ok := readChat(filepath.Join(dir, e.Name()))
		if !ok {
			continue
		}
		if cur, seen := into[id]; !seen || c.at.After(cur.at) {
			into[id] = c
		}
	}
	return nil
}

func readChat(path string) (chat, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return chat{}, false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return chat{}, false
	}
	var c struct {
		Title          string `json:"title"`
		LastActivityAt int64  `json:"lastActivityAt"`
	}
	if json.Unmarshal(raw, &c) != nil {
		return chat{}, false
	}
	at := info.ModTime()
	if c.LastActivityAt > 0 {
		at = time.UnixMilli(c.LastActivityAt)
	}
	return chat{title: c.Title, at: at}, true
}
