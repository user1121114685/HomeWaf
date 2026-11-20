package wafcli

import (
	"HomeWaf/internal/protection/wafpipeline"
	"bytes"
	"context"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type mockSites struct {
	sites  map[uint16]wafpipeline.PortConfig
	writes int
	deny   bool
}

func (s *mockSites) serve(w http.ResponseWriter, r *http.Request) {
	send := func(code int, data any) { _ = json.NewEncoder(w).Encode(map[string]any{"code": code, "data": data}) }
	if s.deny {
		send(-999, nil)
		return
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == portsAPI:
		configs := []wafpipeline.PortConfig{}
		for _, cfg := range s.sites {
			configs = append(configs, cfg)
		}
		send(0, configs)
	case r.Method == http.MethodPost && r.URL.Path == portsAPI+"/save":
		var req struct{ Config wafpipeline.PortConfig }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			send(-1, nil)
			return
		}
		cfg := req.Config
		old := s.sites[cfg.Port]
		if cfg.Revision != old.Revision || old.Revision == 0 && cfg.ListenEnabled {
			send(-1, nil)
			return
		}
		cfg.Revision++
		s.sites[cfg.Port] = cfg
		s.writes++
		send(0, cfg)
	case r.Method == http.MethodPost && r.URL.Path == portsAPI+"/delete":
		var req struct{ Port uint16 }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || s.sites[req.Port].ListenEnabled {
			send(-1, nil)
			return
		}
		delete(s.sites, req.Port)
		s.writes++
		send(0, nil)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func testCommand(t *testing.T, base, input string, args ...string) (int, map[string]any, string) {
	t.Helper()
	old := siteClientFactory
	siteClientFactory = func(wait time.Duration) (*apiClient, error) { return newAPIClient(base, "test-key", wait), nil }
	defer func() { siteClientFactory = old }()
	root := newRootCommand(Deps{})
	var out, stderr bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&stderr)
	root.SetIn(strings.NewReader(input))
	root.SetArgs(args)
	_, err := root.ExecuteC()
	code := 0
	if err != nil {
		code = errorCode(err)
		if !strings.Contains(err.Error(), errResultWritten.Error()) {
			printError(root, err, code)
		}
	}
	var result map[string]any
	if e := json.Unmarshal(out.Bytes(), &result); e != nil {
		t.Fatalf("invalid single JSON output %q: %v", out.String(), e)
	}
	return code, result, out.String()
}

func TestSiteWorkflow(t *testing.T) {
	m := &mockSites{sites: map[uint16]wafpipeline.PortConfig{}}
	srv := httptest.NewServer(http.HandlerFunc(m.serve))
	defer srv.Close()
	in := `{"port":18080,"name":"family","upstream":"http://127.0.0.1:9112","policy":{"defense":{"enabled":true,"mode":"restrict","window_seconds":1800,"count":10,"duration_seconds":3600,"tcp_action":"reset"}}}`
	action := func(in string, want string) {
		t.Helper()
		code, res, _ := testCommand(t, srv.URL, in, "site", "apply", "-f", "-", "-o", "json")
		if code != 0 || res["results"].([]any)[0].(map[string]any)["action"] != want {
			t.Fatalf("code=%d result=%v", code, res)
		}
	}
	action(in, "created")
	action(in, "unchanged")
	if m.writes != 1 {
		t.Fatalf("repeat wrote %d times", m.writes)
	}
	action(`{"port":18080,"name":"changed"}`, "updated")
	if m.sites[18080].Upstream != "http://127.0.0.1:9112" {
		t.Fatal("partial apply erased nickname")
	}
	code, res, _ := testCommand(t, srv.URL, "", "site", "get", "18080", "-o", "json")
	if code != 0 || res["site"].(map[string]any)["name"] != "changed" {
		t.Fatalf("get: %v", res)
	}
	for _, cmd := range []string{"start", "stop", "stop", "delete"} {
		code, _, _ := testCommand(t, srv.URL, "", "site", cmd, "18080", "-o", "json")
		if code != 0 {
			t.Fatalf("%s code %d", cmd, code)
		}
	}
	code, _, _ = testCommand(t, srv.URL, "", "site", "delete", "18080", "-o", "json")
	if code != ExitNotFound {
		t.Fatalf("missing code=%d", code)
	}
	m.deny = true
	code, _, _ = testCommand(t, srv.URL, "", "site", "list", "-o", "json")
	if code != ExitAuth {
		t.Fatalf("auth code=%d", code)
	}
}

func TestBatchPreflight(t *testing.T) {
	m := &mockSites{sites: map[uint16]wafpipeline.PortConfig{}}
	srv := httptest.NewServer(http.HandlerFunc(m.serve))
	defer srv.Close()
	for _, input := range []string{
		`[{"host":"a.example.com","port":18080,"remote_host":"http://127.0.0.1","remote_port":9112},{"code":"x","typo":1}]`,
		`[{"host":"a.example.com","port":18080,"remote_host":"http://127.0.0.1","remote_port":9112},{"host":"b.example.com","port":18081}]`,
	} {
		code, _, _ := testCommand(t, srv.URL, input, "site", "apply", "-f", "-", "-o", "json")
		if code != ExitUsage || m.writes != 0 {
			t.Fatalf("code=%d writes=%d", code, m.writes)
		}
	}
}

func TestClientBoundaries(t *testing.T) {
	t.Run("redirect", func(t *testing.T) {
		hits := 0
		other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }))
		defer other.Close()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, other.URL, 302) }))
		defer srv.Close()
		a := newAPIClient(srv.URL, "test-key", 0)
		err := a.call(context.Background(), "GET", "/", nil, nil)
		if errorCode(err) != ExitFailed || hits != 0 {
			t.Fatalf("followed redirect: %v %d", err, hits)
		}
	})
	t.Run("unavailable", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		base := srv.URL
		srv.Close()
		a := newAPIClient(base, "test-key", 0)
		if err := a.call(context.Background(), "GET", "/", nil, nil); errorCode(err) != ExitUnavailable {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("tls", func(t *testing.T) {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"code":0,"data":{}}`)) }))
		defer srv.Close()
		a := newAPIClient(srv.URL, "test-key", 0)
		if err := a.call(context.Background(), "GET", "/", nil, nil); err == nil {
			t.Fatal("untrusted local server was accepted")
		}
		path := filepath.Join(t.TempDir(), "domain.crt")
		if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}), 0600); err != nil {
			t.Fatal(err)
		}
		config, err := localManagementTLS(path, "")
		if err != nil {
			t.Fatal(err)
		}
		a.http.Transport.(*http.Transport).TLSClientConfig = config
		if err := a.call(context.Background(), "GET", "/", nil, nil); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("proxy disabled", func(t *testing.T) {
		a := newAPIClient("http://127.0.0.1:26666", "test-key", 0)
		if a.http.Transport.(*http.Transport).Proxy != nil {
			t.Fatal("environment proxy is enabled")
		}
	})
}
