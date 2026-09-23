package layout

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Validate checks every chat file in the group before anything is copied, so
// an app update that changes the format stops the sync instead of spreading.
func Validate(g Group) error {
	for _, a := range g.Accounts {
		dir := g.OrgDir(a)
		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, e := range entries {
			kind, id := Classify(e.Name(), e.IsDir(), g.Root.ChatDirs)
			if kind != Chat {
				continue
			}
			path := filepath.Join(dir, e.Name())
			if !e.Type().IsRegular() {
				return fmt.Errorf("%s: not a regular file", path)
			}
			if err := checkChat(path, "local_"+id); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkChat(path, want string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var chat struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(raw, &chat); err != nil {
		return fmt.Errorf("%s: not a chat file: %w", path, err)
	}
	if chat.SessionID != want {
		return fmt.Errorf("%s: sessionId is %q, want %q", path, chat.SessionID, want)
	}
	return nil
}

// CountChats counts the live chats the app would list from one org folder.
func CountChats(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if kind, _ := Classify(e.Name(), e.IsDir(), false); kind == Chat {
			n++
		}
	}
	return n, nil
}
