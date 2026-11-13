package wafproxy

import (
	"HomeWaf/internal/proxyidentity"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestIncidentIdentityAppliedAfterDirectorAndConnection(t *testing.T) {
	seen := make(chan http.Header, 1)
	back := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { seen <- r.Header.Clone(); w.WriteHeader(200) }))
	defer back.Close()
	u, _ := url.Parse(back.URL)
	p := NewSingleHostReverseProxyCustomHeader(u, map[string]string{"X-Forwarded-For": "127.0.0.1", "CF-Connecting-IP": "127.0.0.1", "X-App": "keep"}, nil)
	r := httptest.NewRequest("GET", "http://site.test/", nil)
	r.Header.Set("Connection", "X-Real-IP, X-Forwarded-For, X-Forwarded-Proto")
	r.Header.Set("X-Forwarded-For", "127.0.0.1")
	r.Header.Set("True-Client-IP", "127.0.0.1")
	r.Header.Set("My-Visitor-IP", "127.0.0.1")
	r.Header.Set("X-Original-URL", "/admin")
	r.Header.Set("Authorization", "Bearer app")
	r = r.WithContext(proxyidentity.WithIdentity(r.Context(), proxyidentity.Identity{IP: "198.51.100.8", Proto: "https", Host: r.Host, Port: "443", SourceHeaders: []string{"My-Visitor-IP"}}))
	w := httptest.NewRecorder()
	p.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("status %d", w.Code)
	}
	h := <-seen
	for _, name := range []string{"CF-Connecting-IP", "True-Client-IP", "My-Visitor-IP", "X-Original-URL"} {
		if h.Get(name) != "" {
			t.Errorf("leaked %s", name)
		}
	}
	if h.Get("X-Forwarded-For") != "198.51.100.8" || h.Get("X-Real-IP") != "198.51.100.8" || h.Get("X-Forwarded-Proto") != "https" || h.Get("X-App") != "keep" || h.Get("Authorization") != "Bearer app" {
		t.Fatal(h)
	}
	if r.Header.Get("X-Forwarded-For") != "127.0.0.1" || r.Header.Get("True-Client-IP") != "127.0.0.1" {
		t.Fatal("original evidence mutated")
	}
}
