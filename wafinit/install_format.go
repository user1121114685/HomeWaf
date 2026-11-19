package wafinit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// The directory marker is checked before configuration, keys, or database
// files are opened by either the supervisor or its worker.
const FormatMarker = ".homewaf-format-v3"

// CheckInstallFormat is also used by installers before they stop an existing service.
func CheckInstallFormat(base string) error {
	dir := filepath.Join(base, "data")
	ok, err := hasFormatMarker(dir)
	if err != nil || ok {
		return err
	}
	for _, name := range []string{"data", ".homewaf-keys"} {
		entries, err := os.ReadDir(filepath.Join(base, name))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if len(entries) > 0 {
			// Another process may have just initialized the directory.
			if ok, err := hasFormatMarker(dir); err != nil || ok {
				return err
			}
			return fmt.Errorf("HomeWaf v3 requires a fresh installation directory and database. Existing data in %s must be handled by the administrator", base)
		}
	}
	return nil
}

func hasFormatMarker(dir string) (bool, error) {
	path := filepath.Join(dir, FormatMarker)
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() || info.Size() != 0 {
		return false, fmt.Errorf("invalid HomeWaf installation marker %s", path)
	}
	return true, nil
}

func EnsureInstallFormat(base string) error {
	if err := CheckInstallFormat(base); err != nil {
		return err
	}
	dir := filepath.Join(base, "data")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, FormatMarker), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if errors.Is(err, os.ErrExist) {
		_, err = hasFormatMarker(dir)
		return err
	}
	if err != nil {
		return err
	}
	return f.Close()
}
