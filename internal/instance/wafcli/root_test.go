package wafcli

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestExecuteSingleJSON(t *testing.T) {
	oldArgs, oldOut, oldFactory := os.Args, resultOut, siteClientFactory
	defer func() { os.Args = oldArgs; resultOut = oldOut; siteClientFactory = oldFactory }()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == portsAPI {
			_, _ = w.Write([]byte(`{"code":0,"data":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"code":-1,"msg":"operation failed"}`))
	}))
	defer srv.Close()
	siteClientFactory = func(wait time.Duration) (*apiClient, error) { return newAPIClient(srv.URL, "test-key", wait), nil }
	for _, tc := range []struct {
		args []string
		code int
	}{
		{[]string{"samwaf", "site", "list", "-o", "json"}, 0},
		{[]string{"samwaf", "site", "list", "--bogus", "-o", "json"}, 2},
		{[]string{"samwaf", "site", "get", "42", "-o", "json"}, ExitNotFound},
	} {
		var out bytes.Buffer
		resultOut = &out
		os.Args = tc.args
		code := Execute(Deps{})
		if code != tc.code {
			t.Fatalf("%v: code=%d output=%s", tc.args, code, out.String())
		}
		var v map[string]any
		if err := json.Unmarshal(out.Bytes(), &v); err != nil {
			t.Fatalf("not single json: %s", out.String())
		}
	}
	// Batch results already printed: do not append a second JSON error.
	var out bytes.Buffer
	resultOut = &out
	os.Args = []string{"samwaf", "site", "apply", "-f", "-", "-o", "json"}
	root := newRootCommand(Deps{})
	root.SetIn(strings.NewReader(`{"port":8080,"upstream":"http://127.0.0.1:9112","policy":{"defense":{"enabled":true,"mode":"restrict","window_seconds":60,"count":5,"duration_seconds":60,"tcp_action":"reset"}}}`))
	root.SetArgs(os.Args[1:])
	_, err := root.ExecuteC()
	if err == nil || !errors.Is(err, errResultWritten) {
		t.Fatalf("batch failure=%v", err)
	}
	var v map[string]any
	if json.Unmarshal(out.Bytes(), &v) != nil || v["ok"] != false {
		t.Fatalf("batch JSON=%s", out.String())
	}
}

func TestLegacyRegistration(t *testing.T) {
	root := newRootCommand(Deps{ServiceControl: func(s string) error { return errors.New("failure") }})
	for _, name := range []string{"install", "start", "stop", "restart", "uninstall", "rolling-restart", "resetpwd", "resetotp", "repairdb", "execsql", "migratedb", "rollback"} {
		cmd, _, err := root.Find([]string{name})
		if err != nil || cmd.Name() != name || cmd.Example == "" || cmd.GroupID == "" {
			t.Fatalf("missing %s", name)
		}
	}
	cmd, _, _ := root.Find([]string{"handoff-restart"})
	if !cmd.Hidden {
		t.Fatal("handoff must remain hidden")
	}
	root.SetArgs([]string{"start"})
	if err := root.Execute(); errorCode(err) != ExitFailed {
		t.Fatalf("service error=%v", err)
	}
}
