package proxyidentity

import (
	"net/http"
	"strings"
	"testing"
)

func TestParseChainRequiresEveryHop(t *testing.T) {
	for _, tc := range []struct {
		name   string
		values []string
		valid  bool
		want   string
	}{
		{"repeated fields", []string{"127.0.0.1", "198.51.100.9, 10.0.0.1"}, true, "198.51.100.9"},
		{"invalid suffix", []string{"127.0.0.1, garbage"}, false, ""},
		{"empty hop", []string{"127.0.0.1,"}, false, ""},
		{"unspecified", []string{"0.0.0.0"}, false, ""},
		{"zone", []string{"fe80::1%eth0"}, false, ""},
		{"large", []string{strings.Repeat(" ", 8192) + "127.0.0.1"}, false, ""},
		{"many hops", []string{strings.Repeat("10.0.0.1,", 64) + "127.0.0.1"}, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{"X-Forwarded-For": tc.values}
			ips, ok := ParseChain(h, "X-Forwarded-For")
			if ok != tc.valid {
				t.Fatalf("valid=%v want=%v", ok, tc.valid)
			}
			if tc.want != "" && ips[1].String() != tc.want {
				t.Fatal(ips)
			}
		})
	}
}

func TestIdentityReplacesAllAliasesWithoutTouchingCredentials(t *testing.T) {
	h := http.Header{"x-real-ip": {"127.0.0.1"}, "Cf-Connecting-Ip": {"127.0.0.1"}, "X-Original-Url": {"/admin"}, "My-Visitor": {"127.0.0.1"}, "Authorization": {"Bearer application"}, "Cookie": {"session=application"}}
	Identity{IP: "198.51.100.8", Proto: "https", Host: "site.test:15000", Port: "15000", SourceHeaders: []string{"My-Visitor"}}.Apply(h)
	for _, name := range []string{"x-real-ip", "Cf-Connecting-Ip", "X-Original-Url", "My-Visitor"} {
		if _, ok := h[name]; ok {
			t.Fatalf("alias retained: %s", name)
		}
	}
	if h.Get("X-Real-IP") != "198.51.100.8" || h.Get("X-Forwarded-For") != "198.51.100.8" || h.Get("Authorization") != "Bearer application" || h.Get("Cookie") != "session=application" {
		t.Fatal(h)
	}
}

func TestUpgradeAllowlist(t *testing.T) {
	for _, tc := range []struct {
		values   []string
		rejected bool
	}{
		{nil, false}, {[]string{"websocket"}, false}, {[]string{"WebSocket"}, false},
		{[]string{"h2c"}, true}, {[]string{"websocket, h2c"}, true}, {[]string{"websocket", "h2c"}, true}, {[]string{""}, true},
	} {
		if got := UnsupportedUpgrade(http.Header{"Upgrade": tc.values}); got != tc.rejected {
			t.Fatalf("upgrade %v: rejected=%v", tc.values, got)
		}
	}
}
