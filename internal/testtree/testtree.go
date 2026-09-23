// Package testtree builds fake Claude desktop app folders for tests.
package testtree

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Made-up ids in the app's lowercase UUID form.
const (
	AcctA = "aaaaaaaa-0000-4000-8000-00000000000a"
	AcctB = "bbbbbbbb-0000-4000-8000-00000000000b"
	Org   = "0e0e0e0e-0000-4000-8000-000000000001"
	Chat1 = "c1c1c1c1-0000-4000-8000-000000000001"
	Chat2 = "c2c2c2c2-0000-4000-8000-000000000002"
)

// T0 is the base modification time tests offset from.
var T0 = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

// ChatJSON is the smallest chat file layout.Validate accepts.
func ChatJSON(id, title string) string {
	return `{"sessionId":"local_` + id + `","title":"` + title + `"}`
}

// Write creates path and its parents, then sets its modification time.
func Write(t *testing.T, path, body string, mtime time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

// Mkdir creates a folder and its parents.
func Mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
}

// Read returns a file's body.
func Read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// MTime returns a file's modification time.
func MTime(t *testing.T, path string) time.Time {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.ModTime()
}

// Exists reports whether anything is at path, without following symlinks.
func Exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}
