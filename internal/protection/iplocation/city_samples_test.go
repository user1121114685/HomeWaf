package iplocation

import (
	"context"
	"net/netip"
	"testing"
)

func TestSampleCitiesCoverageAndDisabledFamily(t *testing.T) {
	loadBundledDualStack(t)
	m := NewManager()
	defer m.Close()
	if err := m.ReloadFromConfig(t.TempDir(), "ip2region", "ip2region", "opensource", "opensource"); err != nil {
		t.Fatal(err)
	}
	for _, v6 := range []bool{true, false} {
		rows, cities, err := m.SampleCities(context.Background(), true, v6)
		if err != nil {
			t.Fatal(err)
		}
		if len(cities) < 100 {
			t.Fatalf("unexpectedly sparse city coverage: %d", len(cities))
		}
		seen := map[string]bool{}
		full, missing := 0, 0
		carriers := map[string]bool{}
		for _, row := range rows {
			if seen[row.IP] {
				t.Fatal("duplicate", row.IP)
			}
			seen[row.IP] = true
			ip := netip.MustParseAddr(row.IP)
			last := ip.AsSlice()[len(ip.AsSlice())-1]
			if last < 2 || last == 255 {
				t.Fatal("boundary address", row.IP)
			}
			geo := m.Lookup(row.IP)
			if geo.Country != row.Country || geo.Province != row.Province || geo.City != row.City {
				t.Fatal("invented geography", row)
			}
			carriers[row.ISP] = true
		}
		for _, city := range cities {
			expected6 := 5
			if !v6 {
				expected6 = 1
			}
			if city.IPv4Target != 5 || city.IPv6Target != expected6 || city.IPv4 > 5 || city.IPv6 > expected6 {
				t.Fatal("invalid quota", city)
			}
			if city.IPv4 == 5 && city.IPv6 == expected6 {
				full++
			} else {
				missing++
				if missing <= 8 {
					t.Logf("uncovered: %+v", city)
				}
			}
		}
		if full < 100 {
			for _, city := range cities[:min(12, len(cities))] {
				t.Logf("coverage: %+v", city)
			}
			t.Fatal("too few cities at target", full)
		}
		for _, carrier := range []string{"移动", "联通", "电信"} {
			if !carriers[carrier] {
				t.Fatal("missing carrier", carrier)
			}
		}
		t.Logf("IPv6 enabled=%v cities=%d full=%d incomplete=%d distinct IPs=%d", v6, len(cities), full, missing, len(rows))
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := m.SampleCities(ctx, true, true); err == nil {
		t.Fatal("cancellation ignored")
	}
}
