package iplocation

import (
	"context"
	"net/netip"
	"strings"
	"testing"
)

func TestSampleAddressesUseActiveDatabaseAndIncludeAllowedAndOutside(t *testing.T) {
	loadBundledDualStack(t)
	m := NewManager()
	defer m.Close()
	if err := m.ReloadFromConfig(t.TempDir(), "ip2region", "ip2region", "legacy", "legacy"); err != nil {
		t.Fatal(err)
	}
	wanted := RegionOption{}
	for _, region := range m.Regions() {
		if strings.Contains(region.Province, "重庆") || strings.Contains(strings.ToLower(region.Province), "chongqing") {
			wanted = region
			break
		}
	}
	if wanted.Country == "" {
		t.Fatal("active database has no Chongqing region")
	}
	t.Logf("actual region labels: %+v", wanted)
	rows, err := m.SampleAddresses(context.Background(), []RegionOption{wanted})
	if err != nil {
		t.Fatal(err)
	}
	inside, outside := false, false
	families := map[bool]int{}
	seen := map[string]bool{}
	for _, row := range rows {
		if _, err := netip.ParseAddr(row.IP); err != nil {
			t.Fatal(err)
		}
		ip := netip.MustParseAddr(row.IP)
		families[ip.Is4()]++
		if seen[row.IP] {
			t.Fatal("duplicate source")
		}
		seen[row.IP] = true
		last := ip.AsSlice()[len(ip.AsSlice())-1]
		if last < 2 || last == 255 {
			t.Fatalf("unrealistic range boundary: %s", row.IP)
		}
		actual := m.Lookup(row.IP)
		if actual.Country != row.Country || actual.Province != row.Province {
			t.Fatalf("sample invented geo data: %+v %+v", row, actual)
		}
		if row.Country == wanted.Country && row.Province == wanted.Province {
			inside = true
		} else {
			outside = true
		}
	}
	if !inside || !outside {
		t.Fatalf("missing geographic scenarios: %+v", rows)
	}
	if families[true] != 12 || families[false] != 12 {
		t.Fatalf("family coverage must be 50/50: %+v", families)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := m.SampleAddresses(ctx, nil); err == nil {
		t.Fatal("cancel ignored")
	}
}

func TestSampleAddressesRejectsMissingIPv6InsteadOfFillingWithIPv4(t *testing.T) {
	loadBuiltinV4(t)
	m := NewManager()
	defer m.Close()
	if err := m.ReloadFromConfig(t.TempDir(), "ip2region", "ip2region", "opensource", "opensource"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.SampleAddresses(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "IPv6") {
		t.Fatalf("missing family hidden: %v", err)
	}
}
