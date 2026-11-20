package wafproxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestClientCannotSetInternalXFFControl(t *testing.T) {
	seen := make(chan http.Header, 2)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { seen <- r.Header.Clone(); w.WriteHeader(200) }))
	defer backend.Close()
	target, _ := url.Parse(backend.URL)
	for _, custom := range []bool{false, true} {
		var proxy *ReverseProxy
		if custom {
			proxy = NewSingleHostReverseProxyCustomHeader(target, map[string]string{"X-Forwarded-For": "198.51.100.9"}, nil)
		} else {
			proxy = NewSingleHostReverseProxy(target)
		}
		front := httptest.NewServer(proxy)
		req, _ := http.NewRequest("GET", front.URL, nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.10")
		req.Header.Set("X-Waf-Custom-XFF-Control", "skip-auto-append")
		response, err := front.Client().Do(req)
		if err != nil {
			front.Close()
			t.Fatal(err)
		}
		io.Copy(io.Discard, response.Body)
		response.Body.Close()
		front.Close()
		header := <-seen
		if header.Get("X-Waf-Custom-XFF-Control") != "" {
			t.Fatal("internal header forwarded")
		}
		if custom {
			if header.Get("X-Forwarded-For") != "198.51.100.9" {
				t.Fatal("administrator XFF override lost")
			}
		} else if !strings.HasSuffix(header.Get("X-Forwarded-For"), "127.0.0.1") {
			t.Fatal("client suppressed peer address")
		}
	}
}
