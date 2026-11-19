package wafpipeline

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func TestSharedHoneypotConfigValidation(t *testing.T) {
	base := DefaultHoneypotConfig()
	for _, change := range []func(*HoneypotConfig){
		func(c *HoneypotConfig) { c.MaxConcurrent = 0 },
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
		if ValidateHoneypotConfig(cfg) == nil {
			t.Fatalf("accepted invalid shared config: %+v", cfg)
		}
	}
}

func publishTestHoneypot(t *testing.T, cfg HoneypotConfig) {
	t.Helper()
	PublishSharedHoneypot(&SharedHoneypotConfig{Configured: true, Config: cfg})
	t.Cleanup(func() { PublishSharedHoneypot(nil) })
}

func honeypotSnapshot() *Snapshot {
	s := &Snapshot{Config: PortConfig{Port: 5000, Policy: DefaultPolicy()}, Secrets: PortSecrets{GateKey: strings.Repeat("k", 32)}}
	s.Config.Policy.Defense.Enabled = true
	s.Config.Policy.Defense.Mode = DefenseHoney
	return s
}

func TestHoneypotWeightedLuresAreStableAndEscaped(t *testing.T) {
	s := honeypotSnapshot()
	cfg := DefaultHoneypotConfig()
	cfg.MaxDelayMillis = 0
	cfg.Lures = []HoneypotLure{{ID: "a", Title: "Panel A", Weight: 1}, {ID: "b", Title: "Panel B", Weight: 3}}
	publishTestHoneypot(t, cfg)
	a := 0
	for i := 0; i < 1000; i++ {
		ip := netip.AddrFrom4([4]byte{198, 51, byte(i / 256), byte(i % 256)})
		c := NewRequestContext(httptest.NewRequest("GET", "http://test/", nil), "port:5000", ip, DefaultViewLimits())
		first := selectHoneypotLure(s, c, cfg.Lures)
		c.Request.Header.Set("Cookie", "lure=attacker-controlled")
		if first != selectHoneypotLure(s, c, cfg.Lures) {
			t.Fatal("source selection changed or trusted cookie")
		}
		if first.ID == "a" {
			a++
		}
	}
	if a < 150 || a > 350 {
		t.Fatalf("weight ratio ignored: %d of 1000", a)
	}
	cfg.Lures = []HoneypotLure{{ID: "literal", Title: `<img src=x onerror=alert(1)>`, Message: `{{.Secrets}}<script>alert(1)</script>`, Weight: 1}}
	PublishSharedHoneypot(&SharedHoneypotConfig{Configured: true, Config: cfg})
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
			s := honeypotSnapshot()
			cfg := DefaultHoneypotConfig()
			cfg.MaxDelayMillis = 0
			cfg.ArchiveBody = true
			cfg.ArchiveMaxBytes = tc.limit
			publishTestHoneypot(t, cfg)
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
