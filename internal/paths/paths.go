// Package paths names every file and folder ccdejavu reads or writes.
package paths

import (
	"os"
	"path/filepath"
	"strings"
)

// Paths is rooted at a home folder so tests can point it at a temp dir.
type Paths struct {
	Home string
}

// Default is rooted at the current user's home folder.
func Default() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}
	return Paths{Home: home}, nil
}

// AppSupport is the Claude desktop app's data folder.
func (p Paths) AppSupport() string {
	return filepath.Join(p.Home, "Library", "Application Support", "Claude")
}

// CodeRoot holds the Code tab sidebar index, one folder per account and org.
func (p Paths) CodeRoot() string { return filepath.Join(p.AppSupport(), "claude-code-sessions") }

// CoworkRoot holds the Cowork sidebar index, one folder per account and org.
func (p Paths) CoworkRoot() string {
	return filepath.Join(p.AppSupport(), "local-agent-mode-sessions")
}

// Data is ccdejavu's own folder.
func (p Paths) Data() string { return filepath.Join(p.Home, ".ccdejavu") }

// Backups holds the first-run backup.
func (p Paths) Backups() string { return filepath.Join(p.Data(), "backups") }

// Trash holds chats removed because another account deleted them.
func (p Paths) Trash() string { return filepath.Join(p.Data(), "trash") }

// State records the last sync for `ccdejavu status`.
func (p Paths) State() string { return filepath.Join(p.Data(), "state.json") }

// Lock keeps a manual sync and the watcher from running at once.
func (p Paths) Lock() string { return filepath.Join(p.Data(), "lock") }

// Links records which accounts the user chose to keep in sync.
func (p Paths) Links() string { return filepath.Join(p.Data(), "links.json") }

// ClaudeConfig is the Claude Code CLI's settings file, which names its signed-in account.
func (p Paths) ClaudeConfig() string { return filepath.Join(p.Home, ".claude.json") }

// Logs is where launchd writes the watcher's output.
func (p Paths) Logs() string { return filepath.Join(p.Home, "Library", "Logs", "ccdejavu") }

// LaunchAgents is where per-user launchd definitions live.
func (p Paths) LaunchAgents() string { return filepath.Join(p.Home, "Library", "LaunchAgents") }

// Brief shortens app and home paths so a reason fits on one line.
func (p Paths) Brief(s string) string {
	s = strings.ReplaceAll(s, p.AppSupport()+string(os.PathSeparator), "")
	return strings.ReplaceAll(s, p.Home, "~")
}
