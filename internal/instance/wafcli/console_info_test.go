package wafcli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConsoleReadinessDoesNotFollowTLSRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://unreachable.invalid/", http.StatusMovedPermanently)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := waitForConsole(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
}

func TestConsoleReadinessRejectsUnavailableListener(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := waitForConsole(ctx, server.URL); err == nil {
		t.Fatal("unavailable console reported ready")
	}
}

func TestInitialPasswordIsPrintedOnlyWhenRequested(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "initial_password.txt")
	const content = "Initial password: synthetic-first-login-secret\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	for _, show := range []bool{false, true} {
		cmd := newConsoleInfoCommand(Deps{DataDir: dir})
		var out bytes.Buffer
		cmd.SetOut(&out)
		if show {
			cmd.SetArgs([]string{"--show-initial-password"})
		} else {
			cmd.SetArgs([]string{})
		}
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.String(), content) != show {
			t.Fatalf("initial password visibility did not follow explicit flag (show=%v)", show)
		}
	}
}

func TestInitialPasswordRejectsSpecialFiles(t *testing.T) {
	if err := printInitialPassword(&bytes.Buffer{}, t.TempDir()); err == nil {
		t.Fatal("a directory was accepted as a password file")
	}
}
