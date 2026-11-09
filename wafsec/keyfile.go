package wafsec

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// loadOrCreateKey publishes a fully written key without replacing an existing
// key. Concurrent workers converge on the winner of the exclusive link.
func loadOrCreateKey(path string, size int) ([]byte, error) {
	if key, err := readKeyFile(path, size); !os.IsNotExist(err) {
		return key, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	key := make([]byte, size)
	defer clear(key)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".homewaf-key-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err := restrictKeyFile(f); err != nil {
		return nil, fmt.Errorf("protect key file: %w", err)
	}
	if _, err := f.WriteString(base64.StdEncoding.EncodeToString(key)); err != nil {
		return nil, err
	}
	if err := f.Sync(); err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	if err := os.Link(f.Name(), path); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, fmt.Errorf("publish key file without overwriting: %w", err)
	}
	return readKeyFile(path, size)
}

func readKeyFile(path string, size int) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("key file must be a regular file, not a symlink")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, errors.New("key file changed while opening")
	}
	if err := checkKeyFile(f, opened); err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(io.LimitReader(f, 4097))
	if err != nil || len(raw) > 4096 {
		return nil, errors.New("cannot read bounded key file")
	}
	defer clear(raw)
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil || len(key) != size {
		clear(key)
		return nil, errors.New("invalid key file; restore the original key instead of generating a replacement")
	}
	return key, nil
}
