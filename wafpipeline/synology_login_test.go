package wafpipeline

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSynologyLoginAccountingAndBlocking(t *testing.T) {
	const failure = "{\n  \"error\": { \"code\" : 400 }, \"success\": false\n}"
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		switch r.Header.Get("X-Fixture-Response") {
		case "success":
			io.WriteString(w, `{"success":true}`)
		case "challenge":
			io.WriteString(w, `{"success":false,"error":{"code":403}}`)
		case "missing":
			io.WriteString(w, `{"error":{"code":400}}`)
		case "oversize":
			io.WriteString(w, failure+strings.Repeat(" ", 9000))
		case "gzip":
			w.Header().Set("Content-Encoding", "gzip")
			z := gzip.NewWriter(w)
			_, _ = io.WriteString(z, failure)
			z.Close()
		default:
			io.WriteString(w, failure)
		}
	}, false)
	p := NewPunisher()
	var clock atomic.Int64
	clock.Store(time.Now().Unix())
	p.Now = func() time.Time { return time.Unix(clock.Load(), 0) }
	f.runtime.State = p
	template := BuiltinTemplates()[1]
	f.update(t, func(s *Snapshot) {
		for _, rule := range template.Policy.RuleSet.Rules {
			if rule.Category == synologyAdminCategory || rule.Category == synologyFailureCategory {
				s.Config.Policy.RuleSet.Rules = append(s.Config.Policy.RuleSet.Rules, rule)
			}
		}
		s.Config.Policy.IPGroup.Entries = []IPEntry{{ID: "trusted", Type: "allow", Trust: "trusted", Value: "192.0.2.3"}}
	})
	s := f.current.Load()
	port := PortID(s.Config.Port)
	var notifications int
	var persisted []PunishmentRecord
	p.OnChange = func(change PunishmentChange) {
		if change.Event == "synology_login_lockdown_started" {
			notifications++
		}
	}
	p.Persist = func(rows []PunishmentRecord) error { persisted = append([]PunishmentRecord(nil), rows...); return nil }
	send := func(ip, path, api, method, account, reply string) *httptest.ResponseRecorder {
		t.Helper()
		form := url.Values{"api": {api}, "method": {method}}
		if account != "" {
			form.Set("account", account)
		} else {
			form.Set("__cIpHeRtExT", "opaque-fixture")
		}
		r := httptest.NewRequest("POST", "http://nas.test"+path, strings.NewReader(form.Encode()))
		r.RemoteAddr = ip + ":9000"
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
		r.Header.Set("X-Fixture-Response", reply)
		r = r.WithContext(context.WithValue(r.Context(), http.LocalAddrContextKey, &net.TCPAddr{Port: int(s.Config.Port)}))
		w := httptest.NewRecorder()
		f.runtime.Serve(f.current.Load(), w, r)
		return w
	}
	const entry, auth, adminType = "/webapi/entry.cgi", "SYNO.API.Auth", `"SYNO.API.Auth.Type"`
	for _, tc := range []struct {
		api, method, account, reply string
		status                      int
	}{
		{adminType, `"get"`, `"admin"`, "success", 403},
		{auth, "login", "admin", "", 403},
		{adminType, `"get"`, `"family"`, "success", 200},
		{"SYNO.Foto.Browse.Diff", "list", "admin", "", 200},
		{auth, "logout", "", "", 200},
		{auth, "login", "", "success", 200},
		{auth, "login", "", "challenge", 200},
		{auth, "login", "", "missing", 200},
		{auth, "login", "", "oversize", 200},
	} {
		if w := send("192.0.2.1", entry, tc.api, tc.method, tc.account, tc.reply); w.Code != tc.status {
			t.Fatalf("operation %q %q expected %d, got %d", tc.api, tc.method, tc.status, w.Code)
		}
	}
	// An old failure at exactly the lower boundary no longer contributes.
	send("192.0.2.1", entry, auth, "login", "", "")
	clock.Add(300)
	for i := 0; i < 10; i++ {
		reply := ""
		if i == 9 {
			reply = "gzip"
		}
		if w := send(fmt.Sprintf("192.0.2.%d", 1+i%3), entry+"?api="+auth, auth, "login", "", reply); w.Code != 200 {
			t.Fatalf("failure %d: unexpected status %d", i+1, w.Code)
		}
		if i < 9 && p.synologyRetryAfter(port) != 0 {
			t.Fatal("locked before ten confirmed login failures")
		}
	}
	if notifications != 1 || len(persisted) != 1 || p.synologyRetryAfter(port) != 10800 {
		t.Fatalf("lock transition: notifications=%d records=%v", notifications, persisted)
	}
	before := f.requests.Load()
	for _, ip := range []string{"192.0.2.3", "198.51.100.200"} {
		w := send(ip, entry, auth, "login", "", "success")
		if w.Code != 429 || w.Header().Get("Retry-After") != "10800" || !strings.Contains(w.Body.String(), "暂停") {
			t.Fatalf("new login from %s: %d %s", ip, w.Code, w.Body.String())
		}
	}
	if f.requests.Load() != before || notifications != 1 || p.synologyRetryAfter("port:1") != 0 {
		t.Fatal("locked request forwarded, re-notified, or leaked to another service")
	}
	if w := send("192.0.2.3", entry, "SYNO.Foto.Browse.Diff", "list", "", "success"); w.Code != 200 {
		t.Fatal("browsing blocked")
	}
	restored := NewPunisher()
	restored.Now = p.Now
	if err := restored.Restore(persisted); err != nil || restored.synologyRetryAfter(port) != 10800 {
		t.Fatal("persisted login lock not restored", err)
	}
	if err := restored.Manage(port, persisted[0].ID, "release"); err != nil || restored.synologyRetryAfter(port) != 0 {
		t.Fatal("manual release failed", err)
	}
	clock.Add(10799)
	if w := send("192.0.2.3", entry, auth, "login", "", "success"); w.Code != 429 || w.Header().Get("Retry-After") != "1" {
		t.Fatal("lock extended or ended early")
	}
	clock.Add(1)
	if w := send("192.0.2.3", entry, auth, "login", "", "success"); w.Code != 200 {
		t.Fatal("login failed to resume")
	}
	f.update(t, func(next *Snapshot) {
		next.Config.Policy.RuleSet.ruleByCategory(synologyAdminCategory).Mode = ModeObserve
		next.Config.Policy.RuleSet.ruleByCategory(synologyFailureCategory).Mode = ModeObserve
	})
	if w := send("192.0.2.1", entry, adminType, `"get"`, `"admin"`, "success"); w.Code != 200 {
		t.Fatal("admin observation blocked")
	}
	for i := 0; i < 10; i++ {
		send("192.0.2.1", entry, auth, "login", "", "")
	}
	if p.synologyRetryAfter(port) != 0 || notifications != 1 {
		t.Fatal("observation created another lock")
	}
}

func TestSynologyConcurrentFailuresCreateOneLock(t *testing.T) {
	p := NewPunisher()
	cfg := BuiltinTemplates()[1].Policy
	s := &Snapshot{Config: PortConfig{Port: 53217, Policy: cfg}}
	rule := cfg.RuleSet.ruleByCategory(synologyFailureCategory)
	s.Compiled = &CompiledPolicy{synologyWindow: &synologyFailureWindow{limit: *rule.LoginFailures}}
	p.Lookup = func(string) *Snapshot { return s }
	p.maxRecords, p.perPort = 1, 1
	p.put(PunishmentRecord{ID: "existing-ip-penalty", PortID: PortID(s.Config.Port), Kind: "history"})
	var notices atomic.Int32
	p.OnChange = func(PunishmentChange) { notices.Add(1) }
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p.recordSynologyFailure(s, rule.ID, fmt.Sprint(i))
		}(i)
	}
	wg.Wait()
	if notices.Load() != 1 || len(p.Records(PortID(s.Config.Port))) != 1 {
		t.Fatal("concurrent threshold did not produce exactly one lock")
	}
}

type failingSynologyResponseBody struct{}

func (failingSynologyResponseBody) Read([]byte) (int, error) {
	return 0, errors.New("fixture read failure")
}
func (failingSynologyResponseBody) Close() error { return nil }

func TestSynologyResponseReadFailurePreservesUpstreamError(t *testing.T) {
	hits := 0
	c := &RequestContext{synologyFailure: func() { hits++ }}
	response := &http.Response{
		StatusCode:    http.StatusOK,
		ContentLength: -1,
		Header:        http.Header{"Content-Type": []string{"application/json"}},
		Body:          failingSynologyResponseBody{},
	}
	if err := inspectSynologyResponse(c, response); err != nil {
		t.Fatal("response inspection replaced the upstream response:", err)
	}
	if hits != 0 {
		t.Fatal("an unreadable response was counted as an authentication failure")
	}
	if _, err := io.ReadAll(response.Body); err == nil {
		t.Fatal("the original upstream response read error was lost")
	}
}
