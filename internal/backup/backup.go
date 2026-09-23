// Package backup takes the one-time copy made before ccdejavu first changes anything.
package backup

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/wjames111/ccdejavu/internal/apply"
	"github.com/wjames111/ccdejavu/internal/layout"
)

// Run copies each root in full into dest/<root folder name>, keeping mtimes
// and permissions. Roots that don't exist are skipped.
func Run(roots []layout.Root, dest string) error {
	for _, r := range roots {
		if _, err := os.Stat(r.Dir); errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return err
		}
		if err := copyTree(r.Dir, filepath.Join(dest, filepath.Base(r.Dir))); err != nil {
			return err
		}
	}
	return nil
}

type dirCopy struct {
	src, target string
}

func copyTree(src, dst string) error {
	var dirs []dirCopy
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		// The app is running and can delete a file between the walk seeing it and us reading it.
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		switch {
		case d.IsDir():
			dirs = append(dirs, dirCopy{path, target})
			return os.MkdirAll(target, 0o700)
		case d.Type()&fs.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			} else if err != nil {
				return err
			}
			return os.Symlink(link, target)
		case d.Type().IsRegular():
			if err := apply.CopyFile(path, target); errors.Is(err, fs.ErrNotExist) {
				return nil
			} else {
				return err
			}
		default:
			return nil // sockets and the like aren't worth keeping
		}
	})
	if err != nil {
		return err
	}
	// Fix up folder mode and mtime last: writing files into a folder bumps its
	// mtime, and a source folder's own mode could block writing into it.
	for i := len(dirs) - 1; i >= 0; i-- {
		info, err := os.Stat(dirs[i].src)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return err
		}
		if err := os.Chmod(dirs[i].target, info.Mode().Perm()); err != nil {
			return err
		}
		if err := os.Chtimes(dirs[i].target, info.ModTime(), info.ModTime()); err != nil {
			return err
		}
	}
	return nil
}
