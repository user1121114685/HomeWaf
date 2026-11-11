package threatip

import (
	"HomeWaf/global"
	"HomeWaf/internal/licensing"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSignedThreatDownloadRejectsUntrustedRollbackAndLeaks(t *testing.T) {
	old := global.CurrentSettings()
	global.UpdateSettings(func(c *global.RuntimeSettings) { c.GCONFIG_OUTBOUND_ALLOWED_HOSTS = "127.0.0.1" })
	defer global.UpdateSettings(func(c *global.RuntimeSettings) { *c = old })
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.RWMutex
	var document, payload []byte
	var rawCalls, allCalls atomic.Int64
	var redirect bool
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allCalls.Add(1)
		mu.RLock()
		defer mu.RUnlock()
		if r.URL.Path == "/manifest" {
			if redirect {
				http.Redirect(w, r, "/raw", 302)
				return
			}
			w.Write(document)
			return
		}
		rawCalls.Add(1)
		w.Write(payload)
	}))
	defer server.Close()
	cfg := SignatureConfig{URL: server.URL + "/raw?token=SECRET", ManifestURL: server.URL + "/manifest?token=SECRET", FeedID: "home-feed", KeyID: "fixture", PublicKey: base64.RawURLEncoding.EncodeToString(pub)}
	body := []byte("198.51.100.10\n2001:db8::/48\n")
	sum := sha256.Sum256(body)
	digest := hex.EncodeToString(sum[:])
	set := func(sequence uint64, feed, digest string, expiry int64, wire []byte, signer ed25519.PrivateKey) {
		t.Helper()
		doc, err := licensing.SignThreatSnapshot(licensing.ThreatSnapshotPayload{Schema: 1, FeedID: feed, Sequence: sequence, SHA256: digest, ExpiresAt: expiry}, "fixture", signer)
		if err != nil {
			t.Fatal(err)
		}
		mu.Lock()
		document = doc
		payload = wire
		redirect = false
		mu.Unlock()
	}
	set(10, "home-feed", digest, time.Now().Add(time.Hour).Unix(), body, key)
	raw, manifest, _, err := FetchSignedSnapshot(context.Background(), cfg, 0, "", server.Client())
	if err != nil || string(raw) != string(body) || manifest.Sequence != 10 || rawCalls.Load() != 1 {
		t.Fatalf("valid signed feed rejected: %v", err)
	}
	// The exact same authenticated version can refresh freshness without rollback.
	if _, _, _, err = FetchSignedSnapshot(context.Background(), cfg, 10, digest, server.Client()); err != nil {
		t.Fatal(err)
	}
	_, wrongKey, _ := ed25519.GenerateKey(rand.Reader)
	for _, name := range []string{"wrong-key", "wrong-feed", "expired", "rollback", "same-version-change", "tampered-body", "oversized-manifest", "redirect"} {
		t.Run(name, func(t *testing.T) {
			set(11, "home-feed", digest, time.Now().Add(time.Hour).Unix(), body, key)
			switch name {
			case "wrong-key":
				set(11, "home-feed", digest, time.Now().Add(time.Hour).Unix(), body, wrongKey)
			case "wrong-feed":
				set(11, "different-feed", digest, time.Now().Add(time.Hour).Unix(), body, key)
			case "expired":
				set(11, "home-feed", digest, time.Now().Add(-time.Minute).Unix(), body, key)
			case "rollback":
				set(9, "home-feed", digest, time.Now().Add(time.Hour).Unix(), body, key)
			case "same-version-change":
				set(10, "home-feed", strings.Repeat("a", 64), time.Now().Add(time.Hour).Unix(), body, key)
			case "tampered-body":
				set(11, "home-feed", digest, time.Now().Add(time.Hour).Unix(), []byte("SECRET untrusted content"), key)
			case "oversized-manifest":
				mu.Lock()
				document = []byte(strings.Repeat("SECRET", 6000))
				mu.Unlock()
			case "redirect":
				mu.Lock()
				redirect = true
				mu.Unlock()
			}
			before := rawCalls.Load()
			data, _, _, err := FetchSignedSnapshot(context.Background(), cfg, 10, digest, server.Client())
			if err == nil || data != nil || strings.Contains(err.Error(), "SECRET") || strings.Contains(err.Error(), server.URL) {
				t.Fatalf("unsafe failure: %v", err)
			}
			if name != "tampered-body" && rawCalls.Load() != before {
				t.Fatal("untrusted manifest fetched payload or followed redirect")
			}
		})
	}
	for _, change := range []func(*SignatureConfig){func(c *SignatureConfig) { c.URL = strings.Replace(c.URL, "https:", "http:", 1) }, func(c *SignatureConfig) { c.PublicKey = "" }, func(c *SignatureConfig) { c.ManifestURL = "" }} {
		invalid := cfg
		change(&invalid)
		before := allCalls.Load()
		if _, _, _, err := FetchSignedSnapshot(context.Background(), invalid, 0, "", server.Client()); err == nil || allCalls.Load() != before {
			t.Fatal("invalid trust configuration made an outbound request")
		}
	}
}

func TestThreatSnapshotEntryQuotaDoesNotReturnPartialList(t *testing.T) {
	input := strings.Repeat("198.51.100.10\n", MaxSnapshotEntries+1)
	result, err := ParseByType("cidr_only", strings.NewReader(input), 0)
	if err == nil || len(result.IPs) != 0 {
		t.Fatal("oversized snapshot accepted as a partial update")
	}
}
