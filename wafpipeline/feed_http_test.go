package wafpipeline

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPSFeedProviderMockETagAndRedirectIsolation(t *testing.T) {
	requests := 0
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Query().Get("since") != "v1" {
			t.Error("missing since version")
		}
		if r.Header.Get("If-None-Match") == "same" {
			w.WriteHeader(304)
			return
		}
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "https://other.invalid/feed", 302)
			return
		}
		w.Header().Set("ETag", "next")
		_ = json.NewEncoder(w).Encode(SignedDelta{FeedPayload: FeedPayload{Version: "v2", Sequence: 2}, KeyID: "fixture"})
	}))
	defer s.Close()
	p := &HTTPSFeedProvider{URL: s.URL, Client: s.Client()}
	d, etag, err := p.Fetch(context.Background(), "v1", "")
	if err != nil || etag != "next" || d.Version != "v2" {
		t.Fatal("HTTPS mock failed", err)
	}
	if _, _, err = p.Fetch(context.Background(), "v1", "same"); err != ErrFeedNotModified {
		t.Fatal("304 not handled")
	}
	p.URL = s.URL + "/redirect"
	if _, _, err = p.Fetch(context.Background(), "v1", ""); err == nil {
		t.Fatal("redirect followed")
	}
	p.URL = "http://127.0.0.1/"
	if _, _, err = p.Fetch(context.Background(), "v1", ""); err == nil || requests != 3 {
		t.Fatal("plain HTTP accepted")
	}
}
