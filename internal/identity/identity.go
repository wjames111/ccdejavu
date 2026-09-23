// Package identity finds the email behind an account ID from files that record both.
package identity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wjames111/ccdejavu/internal/layout"
	"github.com/wjames111/ccdejavu/internal/paths"
)

// Known maps account IDs to emails. A pair is trusted by the ID inside its file, not by where the file sits.
func Known(p paths.Paths) map[string]string {
	files, _ := filepath.Glob(filepath.Join(p.CoworkRoot(), "*", "*", "*", ".claude", ".claude.json"))
	files = append(files, p.ClaudeConfig())
	type found struct {
		email string
		mtime time.Time
	}
	best := map[string]found{}
	for _, f := range files {
		id, email, mtime, ok := read(f)
		if !ok {
			continue
		}
		if cur, seen := best[id]; seen && !mtime.After(cur.mtime) {
			continue
		}
		best[id] = found{email: email, mtime: mtime}
	}
	out := make(map[string]string, len(best))
	for id, f := range best {
		out[id] = f.email
	}
	return out
}

func read(path string) (id, email string, mtime time.Time, ok bool) {
	info, err := os.Stat(path)
	if err != nil {
		return "", "", time.Time{}, false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", "", time.Time{}, false
	}
	var cfg struct {
		OAuthAccount struct {
			AccountUUID string `json:"accountUuid"`
			Email       string `json:"emailAddress"`
		} `json:"oauthAccount"`
	}
	if json.Unmarshal(raw, &cfg) != nil {
		return "", "", time.Time{}, false
	}
	id, email = cfg.OAuthAccount.AccountUUID, strings.ToLower(strings.TrimSpace(cfg.OAuthAccount.Email))
	if !layout.IsUUID(id) || email == "" {
		return "", "", time.Time{}, false
	}
	return id, email, info.ModTime(), true
}
