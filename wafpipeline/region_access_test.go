package wafpipeline

import (
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestRegionOnlyAccessAppliesToIPv4AndIPv6WithoutLogin(t *testing.T) {
	for _, tc := range []struct {
		name, ip, province string
		known              bool
		want               Action
	}{
		{"Chongqing IPv4", "203.0.113.1", "重庆", true, Allow},
		{"other IPv4", "203.0.113.2", "四川", true, Deny},
		{"Chongqing IPv6", "240e:1234::1", "重庆", true, Allow},
		{"other IPv6", "240e:1234::2", "四川", true, Deny},
		{"unresolved IPv6", "240e:1234::3", "", false, Deny},
	} {
		t.Run(tc.name, func(t *testing.T) {
			policy := DefaultPolicy()
			policy.Gate.Enabled = false
			policy.IPGroup.UnlistedAction = Deny
			policy.IPGroup.Regions = []Region{{Country: "中国", Province: "重庆"}}
			snapshot := &Snapshot{Config: PortConfig{Schema: 1, Port: 5000, Protocol: "http", Upstream: "http://localhost:8080", Policy: policy}}
			runtime := &Runtime{Geo: NewGeoCache(func(netip.Addr) GeoResult { return GeoResult{Country: "中国", Province: tc.province, OK: tc.known} }, func() uint64 { return 1 })}
			result, err := runtime.Explain(snapshot, httptest.NewRequest("GET", "http://test/", nil), netip.MustParseAddr(tc.ip))
			if err != nil || result.Verdict.Action != tc.want {
				t.Fatalf("expected %s, got %+v (%v)", tc.want, result, err)
			}
		})
	}
}
