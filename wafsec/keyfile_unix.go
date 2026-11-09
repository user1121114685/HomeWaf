//go:build !windows

package wafsec

import (
	"errors"
	"os"
)

func restrictKeyFile(f *os.File) error { return f.Chmod(0600) }

func checkKeyFile(_ *os.File, info os.FileInfo) error {
	if info.Mode().Perm()&0077 != 0 {
		return errors.New("key file must not be accessible by group/other users; set mode 0600")
	}
	return nil
}
