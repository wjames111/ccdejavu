// Package apply carries out a plan.
package apply

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/wjames111/ccdejavu/internal/plan"
)

// Run applies actions in order and stops at the first failure, returning how
// many actions completed before it stopped (len(actions) on success).
func Run(actions []plan.Action) (int, error) {
	for i, a := range actions {
		var err error
		switch a.Op {
		case plan.Copy:
			err = CopyFile(a.Src, a.Dst)
		case plan.Mkdir:
			err = os.MkdirAll(a.Dst, 0o700)
		case plan.Trash:
			err = moveToTrash(a.Src, a.Dst)
		default:
			err = fmt.Errorf("unknown op %d", a.Op)
		}
		if err != nil {
			return i, fmt.Errorf("%s: %w", a, err)
		}
	}
	return len(actions), nil
}

// CopyFile replaces dst with src through a temp file and a rename, so the
// app never reads a half-written chat. dst keeps src's mtime, which stops
// the same file bouncing between accounts on the next pass. If the app wrote
// dst after the plan was scanned, the rename is skipped so that write wins.
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".ccdejavu-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)

	if _, err := io.Copy(tmp, in); err != nil {
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
	if err := os.Chmod(name, info.Mode().Perm()); err != nil {
		return err
	}
	if err := os.Chtimes(name, info.ModTime(), info.ModTime()); err != nil {
		return err
	}
	if cur, err := os.Lstat(dst); err == nil && cur.ModTime().After(info.ModTime()) {
		return nil
	}
	return os.Rename(name, dst)
}

func moveToTrash(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	if _, err := os.Lstat(dst); err == nil {
		return fmt.Errorf("%s already exists", dst)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return os.Rename(src, dst)
}
