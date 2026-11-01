package apply

import (
	"os"

	"golang.org/x/sys/unix"
)

// clone makes name an APFS clone of in. It shares in's blocks until either
// file changes, so a copy takes almost no disk space.
func clone(in *os.File, name string) error {
	if err := unix.Fclonefileat(int(in.Fd()), unix.AT_FDCWD, name, 0); err != nil {
		return err
	}
	out, err := os.Open(name)
	if err != nil {
		return err
	}
	defer out.Close()
	return out.Sync()
}
