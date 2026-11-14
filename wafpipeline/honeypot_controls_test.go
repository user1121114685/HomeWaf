package wafpipeline

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestHoneypotControlValidationAndLegacyDefaults(t *testing.T) {
	base := DefaultPolicy().Honeypot
	for _, change := range []func(*HoneypotConfig){
		func(c *HoneypotConfig) { c.DropAfterHits = -1 },
		func(c *HoneypotConfig) { c.DropSeconds = 7*86400 + 1 },
		func(c *HoneypotConfig) { c.DistributedDrop.DistinctIPs = 1 },
		func(c *HoneypotConfig) { c.DistributedDrop.WindowSeconds = -1 },
		func(c *HoneypotConfig) { c.ArchiveMaxBytes = 65537 },
		func(c *HoneypotConfig) { c.Lures = []HoneypotLure{{ID: "a", Title: "Panel", Weight: 0}} },
		func(c *HoneypotConfig) {
			c.Lures = []HoneypotLure{{ID: "a", Title: "Panel", Weight: 1}, {ID: "a", Title: "Other", Weight: 1}}
		},
		func(c *HoneypotConfig) {
			c.Lures = []HoneypotLure{{ID: "a", Title: "Panel", Message: strings.Repeat("x", 4097), Weight: 1}}
		},
	} {
		cfg := base
		change(&cfg)
		if validateHoneypotControls(cfg) == nil {
			t.Fatalf("accepted invalid controls: %+v", cfg)
		}
	}
	legacy := HoneypotConfig{}
	if err := validateHoneypotControls(legacy); err != nil {
		t.Fatal("legacy policy rejected", err)
	}
	effective := effectiveHoneypot(legacy)
	if effective.DropAfterHits != 0 || effective.DistributedDrop.Enabled || effective.DistributedDrop.DistinctIPs != 10 || effective.DistributedDrop.Seconds != 10800 || effective.DropWindowSeconds != 600 || effective.ArchiveMaxBytes != 16384 {
		t.Fatal("incorrect compatible defaults", effective)
	}
	c := PortConfig{Policy: DefaultPolicy()}
	c.Policy.Honeypot.Lures = []HoneypotLure{{ID: "a", Title: "Panel", Message: "Sign in", Weight: 1}}
	copy := CloneConfig(c)
	copy.Policy.Honeypot.Lures[0].Message = "changed"
	if c.Policy.Honeypot.Lures[0].Message != "Sign in" {
		t.Fatal("lures alias across policy snapshots")
	}
}

func TestHoneypotWeightedLuresAreStableAndEscaped(t *testing.T) {
	s := &Snapshot{Config: PortConfig{Port: 5000, Policy: DefaultPolicy()}, Secrets: PortSecrets{GateKey: strings.Repeat("k", 32)}}
	s.Config.Policy.Honeypot.Lures = []HoneypotLure{{ID: "a", Title: "Panel A", Weight: 1}, {ID: "b", Title: "Panel B", Weight: 3}}
	a := 0
	for i := 0; i < 1000; i++ {
		ip := netip.AddrFrom4([4]byte{198, 51, byte(i / 256), byte(i % 256)})
		c := NewRequestContext(httptest.NewRequest("GET", "http://test/", nil), "port:5000", ip, DefaultViewLimits())
		first := selectHoneypotLure(s, c)
		c.Request.Header.Set("Cookie", "lure=attacker-controlled")
		if first != selectHoneypotLure(s, c) {
			t.Fatal("source selection changed or trusted cookie")
		}
		if first.ID == "a" {
			a++
		}
	}
	if a < 150 || a > 350 {
		t.Fatalf("weight ratio ignored: %d of 1000", a)
	}
	s.Config.Policy.Honeypot.MaxDelayMillis = 0
	s.Config.Policy.Honeypot.Lures = []HoneypotLure{{ID: "literal", Title: `<img src=x onerror=alert(1)>`, Message: `{{.Secrets}}<script>alert(1)</script>`, Weight: 1}}
	c := NewRequestContext(httptest.NewRequest("GET", "http://test/", nil), "port:5000", netip.MustParseAddr("198.51.100.1"), DefaultViewLimits())
	w := httptest.NewRecorder()
	NewHoneyHandler().Respond(s, c, w, Verdict{Action: Honeypot, Stage: 2})
	if strings.Contains(w.Body.String(), "<script>") || strings.Contains(w.Body.String(), "<img ") || !strings.Contains(w.Body.String(), "&lt;script&gt;") || !strings.Contains(w.Body.String(), "{{.Secrets}}") {
		t.Fatal("lure text interpreted as markup or template", w.Body.String())
	}
	if c.Values["honeypot_interaction"].(*HoneypotInteraction).LureID != "literal" {
		t.Fatal("selected lure not archived")
	}
}

func TestHoneypotArchiveIsBoundedRedactedAndInert(t *testing.T) {
	for _, tc := range []struct {
		name, body         string
		limit              int
		omitted, truncated bool
	}{
		{"json", `{"password":"never-log-password","nested":{"token":"never-log-token"},"payload":"<script>not executed</script>"}`, 1024, false, false},
		{"form", `username=probe&password=never-log-password&payload=plain-data`, 1024, false, false},
		{"binary", "\x00\x01\xff", 1024, true, false},
		{"truncated", `{"payload":"` + strings.Repeat("x", 2048) + `"}`, 1024, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &Snapshot{Config: PortConfig{Port: 5000, Policy: DefaultPolicy()}, Secrets: PortSecrets{GateKey: strings.Repeat("k", 32)}}
			s.Config.Policy.Honeypot.MaxDelayMillis = 0
			s.Config.Policy.Honeypot.ArchiveBody = true
			s.Config.Policy.Honeypot.ArchiveMaxBytes = tc.limit
			req := httptest.NewRequest("PUT", "http://test/", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/octet-stream")
			c := NewRequestContext(req, "port:5000", netip.MustParseAddr("198.51.100.1"), DefaultViewLimits())
			w := httptest.NewRecorder()
			NewHoneyHandler().Respond(s, c, w, Verdict{Action: Honeypot, Stage: 2})
			a := c.Values["honeypot_interaction"].(*HoneypotInteraction).Archive
			if a == nil || a.Omitted != tc.omitted || a.Truncated != tc.truncated || a.SampleBytes > tc.limit {
				t.Fatalf("incorrect archive: %+v", a)
			}
			prefix := tc.body
			if len(prefix) > tc.limit {
				prefix = prefix[:tc.limit]
			}
			digest := sha256.Sum256([]byte(prefix))
			if a.PrefixSHA256 != hex.EncodeToString(digest[:]) {
				t.Fatal("evidence digest changed")
			}
			sample, err := base64.StdEncoding.DecodeString(a.BodyBase64)
			if err != nil || strings.Contains(string(sample), "never-log-") || strings.Contains(w.Body.String(), "not executed") {
				t.Fatal("credential leak or submitted data reflected", string(sample))
			}
			if !tc.omitted && !strings.Contains(string(sample), "[MASKED]") {
				t.Fatal("credentials were not masked")
			}
		})
	}
}

func honeyStateFixture(t *testing.T, cfg HoneypotConfig) (*Punisher, *Snapshot, *Runtime, *time.Time) {
	t.Helper()
	now := time.Now()
	p := NewPunisher()
	p.Now = func() time.Time { return now }
	p.start = now
	s := &Snapshot{Config: PortConfig{Schema: 1, Port: 5000, Protocol: "http", Upstream: "http://127.0.0.1:8080", Policy: DefaultPolicy()}}
	s.Config.Policy.Honeypot = cfg
	s.Config.Policy.Honeypot.Enabled = true
	s.Config.Policy.IPGroup.UnlistedAction = Honeypot
	s.Config.Policy.IPGroup.Entries = []IPEntry{{Type: "allow", Trust: "trusted", Value: "198.51.100.99"}}
	r := &Runtime{State: p}
	if err := r.Prepare(s); err != nil {
		t.Fatal(err)
	}
	p.Lookup = func(id string) *Snapshot {
		if id == "port:5000" {
			return s
		}
		return nil
	}
	return p, s, r, &now
}

func honeyVisit(p *Punisher, ip string, n int) {
	p.Submit(HitEvent{ClientIP: netip.MustParseAddr(ip), PortID: "port:5000", Category: "honeypot_visit", RuleID: "honeypot", Severity: Low, ReqID: fmt.Sprintf("%s-%d", ip, n)})
	p.Drain(256)
}

func TestHoneypotPerSourceDropReleaseRestoreAndExpiry(t *testing.T) {
	cfg := DefaultPolicy().Honeypot
	cfg.DropAfterHits = 3
	p, s, r, now := honeyStateFixture(t, cfg)
	ip := netip.MustParseAddr("198.51.100.7")
	for i := 0; i < 2; i++ {
		honeyVisit(p, ip.String(), i)
	}
	if p.HoneypotDropReason("port:5000", ip) != "" {
		t.Fatal("early drop")
	}
	honeyVisit(p, ip.String(), 2)
	if p.HoneypotDropReason("port:5000", ip) == "" || p.HoneypotDropReason("port:5001", ip) != "" {
		t.Fatal("drop missing or crossed port")
	}
	rows := p.Records("port:5000")
	if len(rows) != 1 || rows[0].Kind != "drop" || rows[0].ExpiresAt-now.Unix() != 10800 {
		t.Fatal("incorrect drop duration", rows)
	}
	result, err := r.Explain(s, httptest.NewRequest("GET", "http://test/", nil), ip)
	if err != nil || result.Verdict.Action != Drop {
		t.Fatal("drop did not override honeypot routing", result, err)
	}
	restored := NewPunisher()
	restored.Now = p.Now
	restored.Lookup = p.Lookup
	if err := restored.Restore(rows); err != nil || restored.HoneypotDropReason("port:5000", ip) == "" {
		t.Fatal("drop not restored", err)
	}
	if err := p.Manage("port:5000", rows[0].ID, "release"); err != nil {
		t.Fatal(err)
	}
	honeyVisit(p, ip.String(), 3)
	if p.HoneypotDropReason("port:5000", ip) != "" {
		t.Fatal("release did not reset per-source counter")
	}
	*now = now.Add(10801 * time.Second)
	if restored.HoneypotDropReason("port:5000", ip) != "" {
		t.Fatal("expired drop remains active")
	}
	for _, protected := range []string{"127.0.0.1", "10.0.0.1", "198.51.100.99"} {
		for i := 0; i < 4; i++ {
			honeyVisit(p, protected, i)
		}
		if p.HoneypotDropReason("port:5000", netip.MustParseAddr(protected)) != "" {
			t.Fatal("protected source punished")
		}
	}
}

func TestHoneypotTenSourcesDropThreeHoursAndCooldown(t *testing.T) {
	cfg := DefaultPolicy().Honeypot
	cfg.DistributedDrop.Enabled = true
	p, s, r, now := honeyStateFixture(t, cfg)
	for i := 0; i < 20; i++ {
		honeyVisit(p, "198.51.100.1", i)
	}
	checkIP := netip.MustParseAddr("198.51.100.90")
	if p.HoneypotDropReason("port:5000", checkIP) != "" {
		t.Fatal("one IP counted as multiple sources")
	}
	for i := 2; i <= 9; i++ {
		honeyVisit(p, fmt.Sprintf("198.51.100.%d", i), 1)
	}
	if p.HoneypotDropReason("port:5000", checkIP) != "" {
		t.Fatal("triggered before ten sources")
	}
	honeyVisit(p, "198.51.100.10", 1)
	rows := p.Records("port:5000")
	if len(rows) != 1 || rows[0].Kind != "honeypot_lockdown" || rows[0].Hits != 10 || rows[0].ExpiresAt-now.Unix() != 10800 {
		t.Fatal("wrong distributed lock", rows)
	}
	for _, tc := range []struct {
		ip     string
		action Action
	}{{checkIP.String(), Drop}, {"198.51.100.99", Allow}, {"127.0.0.1", Honeypot}} {
		result, err := r.Explain(s, httptest.NewRequest("GET", "http://test/", nil), netip.MustParseAddr(tc.ip))
		if err != nil || result.Verdict.Action != tc.action {
			t.Fatalf("wrong lock boundary %s: %+v %v", tc.ip, result, err)
		}
	}
	if p.HoneypotDropReason("port:5001", checkIP) != "" {
		t.Fatal("lock crossed ports")
	}
	restored := NewPunisher()
	restored.Now = p.Now
	restored.Lookup = p.Lookup
	if err := restored.Restore(rows); err != nil || restored.HoneypotDropReason("port:5000", checkIP) == "" {
		t.Fatal("lock not restored", err)
	}
	*now = now.Add(10801 * time.Second)
	if restored.HoneypotDropReason("port:5000", checkIP) != "" {
		t.Fatal("three hour expiry ignored")
	}
	if err := p.Manage("port:5000", rows[0].ID, "release"); err != nil {
		t.Fatal(err)
	}
	for i := 20; i < 35; i++ {
		honeyVisit(p, fmt.Sprintf("198.51.100.%d", i), 1)
	}
	if p.HoneypotDropReason("port:5000", checkIP) != "" {
		t.Fatal("manual release lacked cooldown")
	}
	var saved []PunishmentRecord
	p.Persist = func(rows []PunishmentRecord) error { saved = append([]PunishmentRecord(nil), rows...); return nil }
	if err := p.Flush(); err != nil {
		t.Fatal(err)
	}
	restored = NewPunisher()
	restored.Now = p.Now
	restored.Lookup = p.Lookup
	restored.start = *now
	if err := restored.Restore(saved); err != nil {
		t.Fatal(err)
	}
	for i := 40; i < 55; i++ {
		honeyVisit(restored, fmt.Sprintf("198.51.100.%d", i), 1)
	}
	if restored.HoneypotDropReason("port:5000", checkIP) != "" {
		t.Fatal("restart lost cooldown")
	}
}

func TestHoneypotSourceWindowIPv6AndDisabledPunishment(t *testing.T) {
	cfg := DefaultPolicy().Honeypot
	cfg.DistributedDrop.Enabled = true
	cfg.DistributedDrop.DistinctIPs = 2
	p, s, _, now := honeyStateFixture(t, cfg)
	honeyVisit(p, "2001:db8:1::1", 1)
	honeyVisit(p, "2001:db8:1::2", 2)
	if len(p.Records("port:5000")) != 0 {
		t.Fatal("same IPv6 prefix counted twice")
	}
	*now = now.Add(601 * time.Second)
	honeyVisit(p, "2001:db8:2::1", 3)
	if len(p.Records("port:5000")) != 0 {
		t.Fatal("expired distinct source counted")
	}
	s.Config.Policy.HitPolicy.Enabled = false
	honeyVisit(p, "2001:db8:3::1", 4)
	if len(p.Records("port:5000")) != 0 {
		t.Fatal("disabled automatic punishment enforced")
	}
}

func TestHoneypotDropClosesConnectionAndNeverForwards(t *testing.T) {
	p := NewPunisher()
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }, false)
	var checks atomic.Int64
	attachPunisher(t, f, p, &checks)
	results := make(chan Result, 4)
	f.runtime.OnResult = func(_ *Snapshot, _ *RequestContext, r Result) { results <- r }
	f.update(t, func(s *Snapshot) {
		s.Config.Policy.Honeypot.DropAfterHits = 2
		s.Config.Policy.IPGroup.UnlistedAction = Honeypot
	})
	for range 2 {
		if penaltyRequest(t, f, "198.51.100.8", "/admin") != 200 {
			t.Fatal("honeypot did not respond before threshold")
		}
		<-results
		p.Drain(256)
	}
	transport := &http.Transport{DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
	req, _ := http.NewRequest("GET", f.front.URL+"/admin", nil)
	req.Header.Set("X-Forwarded-For", "198.51.100.8")
	response, err := client.Do(req)
	if response != nil {
		response.Body.Close()
	}
	if err == nil {
		t.Fatal("drop returned an HTTP response")
	}
	result := <-results
	if result.Verdict.Action != Drop || result.Honeypot == nil || f.requests.Load() != 0 {
		t.Fatal("drop lost archive or reached upstream", result)
	}
}
