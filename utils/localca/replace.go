package localca

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ReplaceServerCert stages and validates a leaf certificate before updating its policy.
// Root material stays at its canonical paths and is never included in a download of private keys.
func ReplaceServerCert(p Paths, sans []string, commit func() error) error {
	var old = make(map[string][]byte)
	for _, path := range []string{p.SrvCert, p.SrvKey} {
		data, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		old[path] = data
	}
	if err := os.MkdirAll(filepath.Dir(p.SrvCert), 0700); err != nil {
		return err
	}
	dir, err := os.MkdirTemp(filepath.Dir(p.SrvCert), ".certificate-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	stage := p
	stage.SrvCert, stage.SrvKey = filepath.Join(dir, "domain.crt"), filepath.Join(dir, "domain.key")
	if _, err := IssueServerCert(stage, sans, 0); err != nil {
		return err
	}
	if err := CheckServerCert(stage); err != nil {
		return err
	}
	restore := func(cause error) error {
		for _, path := range []string{p.SrvCert, p.SrvKey} {
			var err error
			if old[path] == nil {
				err = removeIfExists(path)
			} else {
				mode := os.FileMode(0644)
				if path == p.SrvKey {
					mode = 0600
				}
				err = writeFileAtomic(path, old[path], mode)
			}
			if err != nil {
				cause = errors.Join(cause, fmt.Errorf("恢复管理证书失败: %w", err))
			}
		}
		return cause
	}
	for _, item := range []struct {
		source, target string
		mode           os.FileMode
	}{{stage.SrvKey, p.SrvKey, 0600}, {stage.SrvCert, p.SrvCert, 0644}} {
		data, err := os.ReadFile(item.source)
		if err != nil {
			return restore(err)
		}
		if err := writeFileAtomic(item.target, data, item.mode); err != nil {
			return restore(err)
		}
	}
	if err := commit(); err != nil {
		return restore(err)
	}
	return nil
}
