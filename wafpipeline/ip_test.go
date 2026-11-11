package wafpipeline

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"
)

type fixtureState struct{ ban, locked, honey bool }

func (s fixtureState) BanReason(string, netip.Addr) string {
	if s.ban {
		return "fixture"
	}
	return ""
}
func (s fixtureState) Locked(string) bool                 { return s.locked }
func (s fixtureState) InHoneypot(string, netip.Addr) bool { return s.honey }
func TestIPPrecedenceScopeExpiryAndUnlisted(t *testing.T) {
	ip := netip.MustParseAddr("203.0.113.1")
	for _, tc := range []struct {
		name     string
		entries  []IPEntry
		unlisted Action
		closed   bool
		want     Action
		reason   string
	}{
		{"closed before trusted", []IPEntry{{Type: "allow", Value: ip.String(), Trust: "trusted"}}, Continue, true, Deny, "access_disabled"},
		{"manual deny before trusted", []IPEntry{{Type: "allow", Value: ip.String(), Trust: "trusted"}, {Type: "deny", Value: ip.String()}}, Continue, false, Deny, "blacklist:custom"},
		{"trusted bypasses shared ban", []IPEntry{{Type: "allow", Value: ip.String(), Trust: "trusted"}}, Continue, false, Allow, "passed"},
		{"allow-only remains subject to ban", []IPEntry{{Type: "allow", Value: ip.String(), Trust: "allow_only"}}, Continue, false, Deny, "blacklist:auto:fixture"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &Snapshot{Config: PortConfig{Schema: 1, Port: 5000, Protocol: "http", Upstream: "http://localhost:8080", Policy: DefaultPolicy()}}
			s.Config.Policy.Access.Enabled = !tc.closed
			s.Config.Policy.IPGroup.Entries = tc.entries
			s.Config.Policy.IPGroup.UnlistedAction = tc.unlisted
			r := &Runtime{State: fixtureState{ban: true, locked: true, honey: true}}
			if err := r.Prepare(s); err != nil {
				t.Fatal(err)
			}
			result, err := r.Explain(s, httptest.NewRequest("GET", "http://test/", nil), ip)
			if err != nil || result.Verdict.Action != tc.want || result.Verdict.Reason != tc.reason {
				t.Fatalf("%+v %v", result, err)
			}
		})
	}
	for _, action := range []Action{Continue, Deny, Honeypot} {
		s := &Snapshot{Config: PortConfig{Schema: 1, Port: 5000, Protocol: "http", Upstream: "http://localhost:8080", Policy: DefaultPolicy()}}
		s.Config.Policy.IPGroup.UnlistedAction = action
		s.Config.Policy.IPGroup.Entries = []IPEntry{{Type: "deny", Value: ip.String(), ExpiresAt: time.Now().Unix() - 1}}
		r := &Runtime{}
		result, err := r.Explain(s, httptest.NewRequest("GET", "http://test/", nil), ip)
		want := action
		if action == Continue {
			want = Allow
		}
		if err != nil || result.Verdict.Action != want {
			t.Fatalf("unlisted %s: %+v %v", action, result, err)
		}
	}
}
func TestClientIdentityAndGeoInvalidation(t *testing.T) {
	cfg := PortConfig{Policy: DefaultPolicy()}
	cfg.Policy.Network.TrustedProxies = []string{"10.0.0.0/8"}
	compiled, _ := compileIP(cfg)
	req := httptest.NewRequest("GET", "http://test/", nil)
	req.RemoteAddr = "198.51.100.1:80"
	req.Header.Set("X-Forwarded-For", "203.0.113.99")
	ip, err := ExtractClientIP(req, cfg.Policy.Network, compiled.Proxies)
	if err != nil || ip.String() != "198.51.100.1" {
		t.Fatal("untrusted header accepted")
	}
	req.RemoteAddr = "10.1.2.3:80"
	req.Header.Set("X-Forwarded-For", "192.0.2.9, ::ffff:203.0.113.8, 10.2.3.4")
	ip, _ = ExtractClientIP(req, cfg.Policy.Network, compiled.Proxies)
	if ip.String() != "203.0.113.8" {
		t.Fatal(ip)
	}
	generation := uint64(1)
	calls := 0
	g := NewGeoCache(func(netip.Addr) GeoResult { calls++; return GeoResult{Country: "Fixture", OK: true} }, func() uint64 { return generation })
	g.Lookup(ip)
	g.Lookup(ip)
	generation++
	g.Lookup(ip)
	if calls != 2 {
		t.Fatal("cache was not invalidated")
	}
	if g.Lookup(netip.MustParseAddr("127.0.0.1")).OK {
		t.Fatal("private IP authorized by region")
	}
}

type fixtureFeed struct {
	delta *SignedDelta
	err   error
	calls int
}

func (p *fixtureFeed) Name() string { return "fixture" }
func (p *fixtureFeed) Fetch(context.Context, string, string) (*SignedDelta, string, error) {
	p.calls++
	return p.delta, "etag", p.err
}
func TestSignedFeedEmptyTrustSignatureOfflineAndExpiry(t *testing.T) {
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	payload := FeedPayload{Version: "1", Sequence: 1, Add: []FeedEntry{{CIDR: "203.0.113.0/24", Score: 90}}}
	delta := &SignedDelta{FeedPayload: payload, KeyID: "fixture"}
	delta.Signature = base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, FeedSigningMessage(payload, "fixture")))
	provider := &fixtureFeed{delta: delta}
	f := NewSignedFeed(func() bool { return true }, nil)
	if err := f.Configure(true, provider, nil, time.Hour); err == nil {
		t.Fatal("empty trust accepted")
	}
	_ = f.Configure(false, provider, nil, time.Hour)
	_ = f.Sync(context.Background(), true)
	if provider.calls != 0 {
		t.Fatal("disabled feed made outbound request")
	}
	if err := f.Configure(true, provider, map[string]ed25519.PublicKey{"fixture": pub}, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := f.Sync(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	ip := netip.MustParseAddr("203.0.113.5")
	if !f.Contains(ip, time.Now().Unix()) {
		t.Fatal("valid feed not published")
	}
	provider.err = errors.New("offline")
	if err := f.Sync(context.Background(), true); err == nil || !f.Contains(ip, time.Now().Unix()) {
		t.Fatal("offline lost valid snapshot")
	}
	provider.err = nil
	provider.delta = &SignedDelta{FeedPayload: FeedPayload{Version: "2", Sequence: 2, Remove: []string{"203.0.113.0/24"}}, KeyID: "fixture", Signature: delta.Signature}
	if err := f.Sync(context.Background(), true); err == nil || !f.Contains(ip, time.Now().Unix()) {
		t.Fatal("invalid signature changed snapshot")
	}
	if f.Contains(netip.MustParseAddr("127.0.0.1"), time.Now().Unix()) {
		t.Fatal("private address blocked by feed")
	}
}
