// Package state records what the last sync did, for `ccdejavu status`.
package state

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Group is the last result for one group of chat folders.
type Group struct {
	Root    string   `json:"root"`
	Name    string   `json:"name"`
	Members []string `json:"members"`
	Actions int      `json:"actions"`
	Skipped string   `json:"skipped,omitempty"`
}

// State is everything ccdejavu remembers between runs.
type State struct {
	Backup   string    `json:"backup,omitempty"` // set once the first-run backup exists
	LastSync time.Time `json:"lastSync"`
	Groups   []Group   `json:"groups"`
}

// Load reads the state file. A missing file is an empty state.
func Load(path string) (State, error) {
	var s State
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	return s, json.Unmarshal(raw, &s)
}

// Save writes the state file atomically so `status` never reads half of it.
func Save(path string, s State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	body, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".state-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(body, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
