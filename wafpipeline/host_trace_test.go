package wafpipeline

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestHostRejectionHasTraceAndMatchesExplainAfterAccessSwitch(t *testing.T) {
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }, false)
	for _, tc := range []struct {
		host    string
		enabled bool
		status  int
		reason  string
	}{{"unknown.test", true, 403, "host_not_allowed"}, {"unknown.test", false, 503, "access_disabled"}, {"family.test", true, 204, "passed"}} {
		f.update(t, func(s *Snapshot) {
			s.Config.HostNames = []string{"family.test"}
			s.Config.Policy.Access.Enabled = tc.enabled
		})
		req, _ := http.NewRequest("GET", f.front.URL+"/", nil)
		req.Host = tc.host
		before := f.requests.Load()
		response, err := f.front.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, response.Body)
		response.Body.Close()
		if response.StatusCode != tc.status {
			t.Fatalf("host=%s access=%v got=%d", tc.host, tc.enabled, response.StatusCode)
		}
		if (f.requests.Load() == before) != (tc.status != 204) {
			t.Fatal("wrong origin reachability")
		}
		sample := httptest.NewRequest("GET", "http://"+tc.host+"/", nil)
		result, err := f.runtime.Explain(f.current.Load(), sample, netip.MustParseAddr("127.0.0.1"))
		if err != nil {
			t.Fatal(err)
		}
		if result.Verdict.Reason != tc.reason {
			t.Fatalf("explanation diverged: %+v", result)
		}
		if tc.status != 204 {
			recorded := f.current.Load().Compiled.Metrics.Last.Load()
			if recorded == nil || recorded.Verdict.Reason != tc.reason || len(recorded.Trace) == 0 {
				t.Fatal("early refusal had no trace")
			}
		}
	}
}
