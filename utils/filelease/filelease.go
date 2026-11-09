package filelease

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Lease is a held file lock. Release is idempotent and closes the lock file.
type Lease struct {
	once sync.Once
	f    *os.File
	path string
	err  error
}

// Release closes the file. Closing the descriptor releases flock/LockFileEx locks.
func (l *Lease) Release() {
	l.once.Do(func() {
		if l.f != nil {
			l.err = l.f.Close()
		}
	})
}

// Err returns the first release/close error observed after a successful acquire.
func (l *Lease) Err() error { return l.err }

var errSymlinkPath = errors.New("file lease path must not be a symlink")

func openLockFile(path string) (*os.File, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	abs = filepath.Clean(abs)
	// Inspect path components instead of comparing EvalSymlinks output: on
	// Windows it also expands 8.3 names and normalizes case for regular paths.
	// Check parents even when the lock file has not been created yet.
	for current := abs; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("%w: %s", errSymlinkPath, path)
		}
		if filepath.Dir(current) == current {
			break
		}
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0700); err != nil {
		return nil, err
	}
	return os.OpenFile(abs, os.O_CREATE|os.O_RDWR, 0600)
}

const pollInterval = 10 * time.Millisecond

func wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(pollInterval):
		return nil
	}
}

// AcquireShared waits for a shared lock until ctx is done.
func AcquireShared(ctx context.Context, path string) (*Lease, error) {
	return acquire(ctx, path, false)
}

// AcquireExclusive waits for an exclusive lock until ctx is done.
func AcquireExclusive(ctx context.Context, path string) (*Lease, error) {
	return acquire(ctx, path, true)
}
