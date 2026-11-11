package wafpipeline

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func TestProtectedFeedOverlapAndDetectorFailureRemainNonPunitive(t *testing.T) {
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }, false)
	feed := NewSignedFeed(nil, nil)
	if err := feed.Restore(FeedCache{Entries: []FeedEntry{{CIDR: "127.0.0.0/8"}}}); err != nil {
		t.Fatal(err)
	}
	feed.enabled.Store(true)
	if feed.Contains(netip.MustParseAddr("127.0.0.1"), 0) {
		t.Fatal("ordinary feed lookup lost protected default")
	}
	f.runtime.Feed = feed
	f.runtime.State = fixtureState{ban: true, honey: true}
	f.update(t, func(s *Snapshot) { s.Config.Policy.IPGroup.FeedEnabled = true })
	for _, trusted := range []bool{false, true} {
		f.update(t, func(s *Snapshot) {
			if trusted {
				s.Config.Policy.IPGroup.Entries = []IPEntry{{Type: "allow", Value: "127.0.0.1", Trust: "trusted"}}
			}
		})
		before := f.requests.Load()
		eventsBefore := len(f.sink.snapshot())
		if status, _, _ := proxyCall(t, f, "GET", "/files", "", nil); status != 204 || f.requests.Load() != before+1 {
			t.Fatal("protected overlap blocked business")
		}
		events := f.sink.snapshot()[eventsBefore:]
		if len(events) != 2 {
			t.Fatalf("missing protected overlap events: %+v", events)
		}
		for _, event := range events {
			if event.Category != "protected_match" || !event.Observe || event.Trusted != trusted {
				t.Fatal("incorrect protected diagnostic")
			}
		}
	}
	f.runtime.State = nil
	f.runtime.Feed = nil
	f.runtime.RuleSteps = func(*Snapshot) []Step {
		return []Step{{Number: 3, Name: "faulty", Inspect: func(c *RequestContext) (Verdict, error) {
			c.Hit("sqli", "partial", High, false, "incomplete detector result")
			return Verdict{Action: Deny}, errors.New("private/path/SECRET")
		}}}
	}
	f.update(t, func(s *Snapshot) { s.Config.Policy.IPGroup.Entries = nil; s.Config.Policy.IPGroup.FeedEnabled = false })
	before := f.requests.Load()
	eventsBefore := len(f.sink.snapshot())
	if status, _, _ := proxyCall(t, f, "GET", "/files", "", nil); status != 204 || f.requests.Load() != before+1 {
		t.Fatal("failed detector blocked origin")
	}
	for _, event := range f.sink.snapshot()[eventsBefore:] {
		if !event.Observe {
			t.Fatal("partial detector failure could punish a visitor")
		}
	}
	result := f.current.Load().Compiled.Metrics.Last.Load()
	raw, _ := json.Marshal(result)
	if strings.Contains(string(raw), "SECRET") || strings.Contains(string(raw), "private/path") {
		t.Fatal("detector error leaked internal details")
	}
}

func TestProtectedRestrictionFastPathPreservesScopeAndExpiry(t *testing.T) {
	p := NewPunisher()
	now := time.Now()
	clockCalls := 0
	p.Now = func() time.Time { clockCalls++; return now }
	if p.ProtectedRestriction("port:5000", netip.MustParseAddr("127.0.0.1")) || clockCalls != 0 {
		t.Fatal("empty state consulted clock or invented a match")
	}
	rows := []PunishmentRecord{{Kind: "ban", Scope: "global", PortID: "port:5000", Prefix: "127.0.0.1/32", Reason: "fixture", ExpiresAt: now.Add(time.Minute).Unix()}, {Kind: "honeypot", Scope: "port", PortID: "port:5000", Prefix: "127.0.0.2/32", Reason: "fixture", ExpiresAt: now.Add(time.Minute).Unix()}}
	for i := range rows {
		rows[i].ID = recordKey(rows[i].Kind, rows[i].Scope, rows[i].PortID, rows[i].Prefix)
	}
	if err := p.Restore(rows); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		port, ip string
		want     bool
	}{{"port:5001", "127.0.0.1", true}, {"port:5000", "127.0.0.2", true}, {"port:5001", "127.0.0.2", false}} {
		if p.ProtectedRestriction(tc.port, netip.MustParseAddr(tc.ip)) != tc.want {
			t.Fatal("diagnostic query changed state scope")
		}
	}
	now = now.Add(2 * time.Minute)
	if p.ProtectedRestriction("port:5000", netip.MustParseAddr("127.0.0.1")) || p.ProtectedRestriction("port:5000", netip.MustParseAddr("127.0.0.2")) {
		t.Fatal("diagnostic query kept expired state")
	}
}
