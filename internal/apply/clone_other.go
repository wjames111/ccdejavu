//go:build !darwin

package apply

import (
	"errors"
	"os"
)

func clone(*os.File, string) error {
	return errors.ErrUnsupported
}
