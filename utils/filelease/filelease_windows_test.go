//go:build windows

package filelease

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestFileLeaseWindowsPathAliases(t *testing.T) {
	for _, name := range []string{"short", "case"} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "Long Directory Name")
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			alias := strings.ToLower(dir)
			if name == "short" {
				ptr, err := windows.UTF16PtrFromString(dir)
				if err != nil {
					t.Fatal(err)
				}
				buf := make([]uint16, len(dir)+1)
				n, err := windows.GetShortPathName(ptr, &buf[0], uint32(len(buf)))
				if err != nil {
					t.Fatal(err)
				}
				if n >= uint32(len(buf)) {
					t.Fatal("short path buffer too small")
				}
				alias = windows.UTF16ToString(buf[:n])
				if strings.EqualFold(alias, dir) {
					t.Skip("8.3 short names are disabled on the test volume")
				}
			}
			path := filepath.Join(alias, "lease.lock")
			// The first acquire creates the file; the second must also accept
			// its alias after the file exists, as on the Windows CI runner.
			for i := 0; i < 2; i++ {
				lease, err := AcquireShared(context.Background(), path)
				if err != nil {
					t.Fatalf("acquire %d via %q: %v", i+1, path, err)
				}
				lease.Release()
			}
			lease, err := AcquireExclusive(context.Background(), filepath.Join(dir, "lease.lock"))
			if err != nil {
				t.Fatal(err)
			}
			defer lease.Release()
			ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
			defer cancel()
			if other, err := AcquireShared(ctx, path); !errors.Is(err, context.DeadlineExceeded) {
				if other != nil {
					other.Release()
				}
				t.Fatalf("alias must share the same lock: %v", err)
			}
		})
	}
}
