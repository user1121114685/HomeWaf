package wafpipeline

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func defenseTestSnapshot(port uint16, mode string, count int) *Snapshot {
	d := DefaultDefense()
	d.Enabled = true
	d.Mode = mode
	d.Count = count
	p := DefaultPolicy()
	p.Defense = &d
	return &Snapshot{Config: PortConfig{Schema: 1, Port: port, Revision: 1, ListenEnabled: true, Protocol: "http", Upstream: "http://127.0.0.1:1", Policy: p}}
}
func defenseTestContext(ip string) *RequestContext {
	return NewRequestContext(httptest.NewRequest("GET", "http://fixture/", nil), "port:5000", netip.MustParseAddr(ip), DefaultViewLimits())
}

var defenseDenied = Verdict{Action: Deny, Status: 403, Reason: "blacklist:custom"}

func TestDefensePortWindowCountsRequestsAcrossIPs(t *testing.T) {
	e := NewDefenseEngine()
	now := time.Now()
	e.Now = func() time.Time { return now }
	_ = e.LoadHistory(nil, nil)
	s := defenseTestSnapshot(5000, DefenseQuiet, 10)
	for i := 0; i < 9; i++ {
		c := defenseTestContext(fmt.Sprintf("198.51.100.%d", i+1))
		c.Hit("auth_fail", "gate", Low, false, "failure")
		c.Hit("weak_pwd", "gate", Low, false, "weak")
		e.Observe(s, c, defenseDenied)
		e.Observe(s, c, defenseDenied)
	}
	e.Observe(s, defenseTestContext("203.0.113.1"), Verdict{Action: Allow, Status: 200, Reason: "upstream_response"})
	if len(e.Pending()) != 0 || e.Status(s).Failures != 9 {
		t.Fatal("duplicate signals counted or success cleared the counter")
	}
	e.Observe(defenseTestSnapshot(5001, DefenseQuiet, 10), defenseTestContext("192.0.2.1"), defenseDenied)
	e.Observe(s, defenseTestContext("192.0.2.2"), defenseDenied)
	items := e.Pending()
	if len(items) != 1 || items[0].Port != 5000 || items[0].State.Mode != DefenseQuiet {
		t.Fatalf("wrong transition: %+v", items)
	}
	deadline := items[0].State.ExpiresAt
	now = now.Add(time.Minute)
	e.Observe(s, defenseTestContext("192.0.2.3"), defenseDenied)
	if e.Pending()[0].State.ExpiresAt != deadline {
		t.Fatal("continued failures extended the restriction")
	}
	e.Reset(5000)
	e.Observe(s, defenseTestContext("192.0.2.4"), defenseDenied)
	now = now.Add(30 * time.Minute)
	e.Observe(s, defenseTestContext("192.0.2.5"), defenseDenied)
	if e.Status(s).Failures != 1 {
		t.Fatal("expired boundary was included")
	}
}

func TestDefenseExcludesOrdinaryErrorsChallengesAndObservation(t *testing.T) {
	e := NewDefenseEngine()
	s := defenseTestSnapshot(5000, DefenseClose, 1)
	for _, v := range []Verdict{{Action: Allow, Status: 404, Reason: "upstream_response"}, {Action: Allow, Status: 500, Reason: "upstream_response"}, {Action: Deny, Status: 503, Reason: "policy_unavailable"}, {Action: Challenge, Status: 401, Reason: "gate:required"}, {Action: Deny, Status: 403, Reason: "defense:unknown_network"}, {Action: Deny, Status: 503, Reason: "access_disabled"}, {Action: Deny, Status: 403, Observe: true, Reason: "blacklist:custom"}} {
		e.Observe(s, defenseTestContext("192.0.2.1"), v)
	}
	if len(e.Pending()) != 0 {
		t.Fatal("ordinary traffic triggered protection")
	}
	c := defenseTestContext("192.0.2.1")
	c.Hit("auth_fail", "gate", Low, false, "failure")
	e.Observe(s, c, Verdict{Action: Challenge, Status: 401, Reason: "gate:basic_failed"})
	if len(e.Pending()) != 1 {
		t.Fatal("actual authentication failure was missed")
	}
}

func TestDefenseHistoryPrefixesExpiryEmptyAndFailure(t *testing.T) {
	e := NewDefenseEngine()
	now := time.Now()
	e.Now = func() time.Time { return now }
	s := defenseTestSnapshot(5000, DefenseRestrict, 1)
	if err := e.LoadHistory([]SuccessPrefix{{Port: 5000, Prefix: "192.0.0.0/16", LastSeen: now.Unix() - SuccessHistorySeconds}}, nil); err != nil {
		t.Fatal(err)
	}
	for _, ip := range []string{"198.51.100.1", "198.51.220.2", "2001:db8:abcd:1234::1"} {
		e.Observe(s, defenseTestContext(ip), Verdict{Action: Allow, Status: 200, Reason: "upstream_response"})
	}
	c := defenseTestContext("203.0.113.1")
	c.Hit("scanner", "scanner", Low, true, "observed")
	e.Observe(s, c, Verdict{Action: Allow, Status: 200, Reason: "upstream_response"})
	e.Observe(s, defenseTestContext("192.0.2.9"), defenseDenied)
	state := e.Pending()[0].State
	if state.Mode != DefenseRestrict || len(state.Prefixes) != 2 {
		t.Fatalf("wrong allow scope: %+v", state)
	}
	s.Config.DefenseState = &state
	for _, tc := range []struct {
		ip   string
		want Action
	}{{"198.51.1.99", Continue}, {"2001:db8:abcd:1234::99", Continue}, {"2001:db8:abcd:1235::1", Deny}, {"203.0.113.1", Deny}} {
		v, err := e.Step(s).Inspect(defenseTestContext(tc.ip))
		if err != nil || v.Action != tc.want {
			t.Fatalf("%s: %+v %v", tc.ip, v, err)
		}
	}
	empty := NewDefenseEngine()
	_ = empty.LoadHistory(nil, nil)
	s.Config.DefenseState = nil
	empty.Observe(s, defenseTestContext("192.0.2.1"), defenseDenied)
	if empty.Pending()[0].State.Reason != "no_success_history" {
		t.Fatal("empty history did not close access")
	}
	broken := NewDefenseEngine()
	_ = broken.LoadHistory(nil, errors.New("unavailable"))
	broken.HistorySaved(nil, nil)
	broken.Observe(s, defenseTestContext("192.0.2.2"), defenseDenied)
	if len(broken.Pending()) != 0 || broken.Status(s).HistoryReady {
		t.Fatal("failed read turned into empty successful history")
	}
}

func TestDefenseRealRequestsKeepNormalLoginAndHideFailedBusinessResponse(t *testing.T) {
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			w.Header().Set("Set-Cookie", "backend_secret=must-not-leak")
			_, _ = io.WriteString(w, "wrong credentials, detailed backend hint")
			return
		}
		_, _ = io.WriteString(w, "normal business")
	}, false)
	old := f.current.Load()
	s := &Snapshot{Config: CloneConfig(old.Config), Secrets: old.Secrets}
	d := DefaultDefense()
	d.Enabled = true
	s.Config.Policy.Defense = &d
	s.Config.Policy.HitPolicy.BusinessLogin = []LoginObservation{{Path: "/login", Method: "POST", SuccessStatuses: []int{200}, SuccessBodyContains: "login succeeded", BodyPrefixBytes: 1024}}
	s.Config.DefenseState = &DefenseState{ID: "fixture", Mode: DefenseQuiet, StartedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(time.Hour).Unix()}
	f.runtime.Defense = NewDefenseEngine()
	_ = f.runtime.Defense.LoadHistory(nil, nil)
	if err := f.runtime.Prepare(s); err != nil {
		t.Fatal(err)
	}
	f.current.Store(s)
	resp, err := f.front.Client().Get(f.front.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "normal business" {
		t.Fatal("normal visitor was blocked")
	}
	resp, err = f.front.Client().Post(f.front.URL+"/login", "application/x-www-form-urlencoded", strings.NewReader("username=someone&password=wrong"))
	if err == nil {
		defer resp.Body.Close()
		t.Fatalf("failed login emitted an HTTP response: %d", resp.StatusCode)
	}
	shared := SharedHoneypotConfig{Configured: true, Revision: 1, Config: DefaultHoneypotConfig()}
	shared.Config.MaxDelayMillis = 0
	PublishSharedHoneypot(&shared)
	t.Cleanup(func() { PublishSharedHoneypot(nil) })
	next := &Snapshot{Config: CloneConfig(s.Config), Secrets: s.Secrets}
	next.Config.Policy.Defense.Mode = DefenseHoney
	next.Config.DefenseState = nil
	honey := NewHoneyHandler()
	f.runtime.Respond = honey.Respond
	if err := f.runtime.Prepare(next); err != nil {
		t.Fatal(err)
	}
	f.current.Store(next)
	resp, err = f.front.Client().Post(f.front.URL+"/login", "application/x-www-form-urlencoded", strings.NewReader("username=someone&password=wrong"))
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "管理控制台") || strings.Contains(string(body), "backend hint") || resp.Header.Get("Set-Cookie") != "" {
		t.Fatal("honeypot leaked the backend response")
	}
}

func TestDefensePreservesNormalGateChallengeButAbortsWrongCredentials(t *testing.T) {
	f := newProxyFixture(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }, false)
	old := f.current.Load()
	s := &Snapshot{Config: CloneConfig(old.Config), Secrets: old.Secrets}
	d := DefaultDefense()
	d.Enabled = true
	s.Config.Policy.Defense = &d
	s.Config.Policy.Gate.Enabled = true
	s.Config.Policy.Gate.Mode = "basic"
	s.Config.DefenseState = &DefenseState{ID: "quiet-fixture", Mode: DefenseQuiet, StartedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(time.Hour).Unix()}
	if err := f.runtime.Prepare(s); err != nil {
		t.Fatal(err)
	}
	f.current.Store(s)
	response, err := f.front.Client().Get(f.front.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 401 || response.Header.Get("WWW-Authenticate") == "" {
		t.Fatal("normal login challenge was suppressed")
	}
	req, _ := http.NewRequest("GET", f.front.URL+"/", nil)
	req.SetBasicAuth("unknown", "incorrect")
	response, err = f.front.Client().Do(req)
	if err == nil {
		response.Body.Close()
		t.Fatal("wrong credentials received an HTTP hint")
	}
	if f.requests.Load() != 0 {
		t.Fatal("gate failure reached the business upstream")
	}
}

func TestDefenseFormVerificationAndServiceTokenFailures(t *testing.T) {
	for _, mode := range []string{"form", "token"} {
		t.Run(mode, func(t *testing.T) {
			f := newProxyFixture(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }, false)
			old := f.current.Load()
			s := &Snapshot{Config: CloneConfig(old.Config), Secrets: old.Secrets}
			d := DefaultDefense()
			d.Enabled = true
			s.Config.Policy.Defense = &d
			s.Config.Policy.Gate.Enabled = true
			if mode == "token" {
				s.Config.Policy.Gate.Enabled = false
				s.Config.Policy.Gate.RequireToken = true
				s.Config.Policy.Gate.TokenHeader = "X-Service-Token"
				s.Secrets.ServiceTokenHashes = []string{tokenHash("fixture-valid-token")}
			}
			s.Config.DefenseState = &DefenseState{ID: "quiet", Mode: DefenseQuiet, StartedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(time.Hour).Unix()}
			if err := f.runtime.Prepare(s); err != nil {
				t.Fatal(err)
			}
			f.current.Store(s)
			var request *http.Request
			if mode == "form" {
				response, err := f.front.Client().Get(f.front.URL + s.Config.Policy.Gate.Prefix + "/login")
				if err != nil {
					t.Fatal(err)
				}
				response.Body.Close()
				if response.StatusCode != 200 {
					t.Fatal("normal login page unavailable")
				}
				request, _ = http.NewRequest("POST", f.front.URL+s.Config.Policy.Gate.Prefix+"/login", strings.NewReader("username=guest&password=wrong"))
				request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			} else {
				valid, _ := http.NewRequest("GET", f.front.URL+"/", nil)
				valid.Header.Set("X-Service-Token", "fixture-valid-token")
				response, err := f.front.Client().Do(valid)
				if err != nil {
					t.Fatal(err)
				}
				response.Body.Close()
				if response.StatusCode != 204 {
					t.Fatal("valid token was blocked")
				}
				request, _ = http.NewRequest("GET", f.front.URL+"/", nil)
				request.Header.Set("X-Service-Token", "invalid")
			}
			response, err := f.front.Client().Do(request)
			if err == nil {
				response.Body.Close()
				t.Fatal("security verification failure leaked an HTTP hint")
			}
		})
	}
}

func TestDefenseLearnsSuccessfulWebSocketBeforeConnectionCloses(t *testing.T) {
	upgrader := websocket.Upgrader{}
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_, _, _ = conn.ReadMessage()
	}, false)
	f.runtime.Defense = NewDefenseEngine()
	_ = f.runtime.Defense.LoadHistory(nil, nil)
	f.current.Store(f.current.Load())
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(f.front.URL, "http")+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if f.runtime.Defense.Status(f.current.Load()).SuccessPrefixes != 1 {
		t.Fatal("successful open WebSocket was absent from the learned scope")
	}
}

func TestDefenseConcurrentFailuresProduceOneIncident(t *testing.T) {
	e := NewDefenseEngine()
	_ = e.LoadHistory(nil, nil)
	s := defenseTestSnapshot(5000, DefenseQuiet, 10)
	var workers sync.WaitGroup
	for i := 0; i < 100; i++ {
		workers.Add(1)
		go func(i int) {
			defer workers.Done()
			e.Observe(s, defenseTestContext(fmt.Sprintf("198.51.100.%d", i+1)), defenseDenied)
		}(i)
	}
	workers.Wait()
	if len(e.Pending()) != 1 || e.Status(s).Failures != 10 {
		t.Fatal("concurrent requests duplicated or lost the threshold transition")
	}
}
