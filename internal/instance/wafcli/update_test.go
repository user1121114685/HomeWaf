package wafcli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCLIUpdateUsesAuthenticatedCanonicalAPIAndWaitsForVersion(t *testing.T) {
	oldFactory, oldOut := updateClientFactory, resultOut
	t.Cleanup(func() { updateClientFactory, resultOut = oldFactory, oldOut })
	for _, scenario := range []string{"success", "current", "blocked", "mismatch", "failed"} {
		t.Run(scenario, func(t *testing.T) {
			updates := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("X-API-Key") != "fixture-key" {
					t.Error("missing local API authentication")
				}
				var data any
				switch r.URL.Path {
				case "/api/v1/sysinfo/checkversion":
					data = map[string]any{"version": "v2.0.2", "version_new": "v2.0.3", "need_update": scenario != "current", "self_update_allowed": scenario != "blocked"}
				case "/api/v1/sysinfo/update":
					updates++
					if r.URL.Query().Get("channel") != "official" {
						t.Error("wrong update channel")
					}
					data = map[string]any{}
				case "/api/v1/sysinfo/version":
					version := "v2.0.3"
					if scenario == "failed" {
						version = "v2.0.2"
					}
					data = map[string]any{"version": version}
				case "/api/v1/sysinfo/updateprogress":
					data = map[string]any{"state": "failed", "error": "fixture checksum failure"}
				default:
					t.Errorf("unexpected request %s", r.URL.Path)
				}
				json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": data})
			}))
			defer server.Close()
			updateClientFactory = func(time.Duration) (*apiClient, error) { return newAPIClient(server.URL, "fixture-key", 0), nil }
			var out bytes.Buffer
			resultOut = &out
			root := newRootCommand(Deps{})
			args := []string{"update", "--wait", "2s", "-o", "json"}
			if scenario == "mismatch" {
				args = append(args, "--version", "v2.0.4")
			}
			root.SetArgs(args)
			_, err := root.ExecuteC()
			wantError := scenario == "blocked" || scenario == "mismatch" || scenario == "failed"
			if (err != nil) != wantError {
				t.Fatalf("error=%v output=%s", err, out.String())
			}
			wantUpdates := 0
			if scenario == "success" || scenario == "failed" {
				wantUpdates = 1
			}
			if updates != wantUpdates {
				t.Fatalf("submitted %d updates, want %d", updates, wantUpdates)
			}
			if strings.Contains(out.String(), "fixture-key") {
				t.Fatal("credential leaked")
			}
		})
	}
}
