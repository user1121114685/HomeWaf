package iplocation

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lionsoul2014/ip2region/binding/golang/xdb"
)

func loadBundledDualStack(t *testing.T) []byte {
	t.Helper()
	v4 := loadBuiltinV4(t)
	v6, err := os.ReadFile("../cmd/homewaf/exedata/ip2region_v6.xdb")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(v6)
	if hex.EncodeToString(sum[:]) != "1f0be1ffbbc3e4eeef10f354e01355f22f57ec649a496a17fd602539f3d384c1" {
		t.Fatal("bundled IPv6 dataset differs from pinned source")
	}
	SetBuiltinData(v4, nil, v6)
	t.Cleanup(func() { SetBuiltinData(v4, nil) })
	return v6
}

func TestFreshInstallHasOfflineIPv6Regions(t *testing.T) {
	v6 := loadBundledDualStack(t)
	m := NewManager()
	defer m.Close()
	// A stale legacy setting must not misparse the known bundled open-source data.
	if err := m.ReloadFromConfig(t.TempDir(), "ip2region", "ip2region", "opensource", "legacy"); err != nil {
		t.Fatal(err)
	}
	status := m.GetStatus()
	if !status.IPv6Builtin || status.IPv6FileSize != int64(len(v6)) || status.IPv6Format != string(FormatOpenSource) {
		t.Fatalf("wrong built-in status: %+v", status)
	}
	if !HasBuiltinFile("ip2region_v6") || !HasBuiltinSource("ipv6", "ip2region") {
		t.Fatal("IPv6 must be available on a fresh install")
	}
	if r := m.Lookup("2001:4860:4860::8888"); r.Unresolved || r.Country == "" {
		t.Fatalf("IPv6 lookup failed: %+v", r)
	}
	matching := false
	for _, v4 := range m.v4Regions {
		if !strings.Contains(v4.Province, "重庆") {
			continue
		}
		for _, v6 := range m.v6Regions {
			if v4 == v6 {
				matching = true
			}
		}
	}
	if !matching {
		t.Fatal("bundled IPv4 and IPv6 must share a selectable Chongqing region")
	}
	header, err := xdb.LoadHeaderFromBuff(v6)
	if err != nil {
		t.Fatal(err)
	}
	for at := int(header.StartIndexPtr); at <= int(header.EndIndexPtr); at += xdb.IPv6.SegmentIndexSize {
		offset := at + 32
		length := int(binary.LittleEndian.Uint16(v6[offset:]))
		ptr := int(binary.LittleEndian.Uint32(v6[offset+2:]))
		if !strings.Contains(string(v6[ptr:ptr+length]), "重庆") {
			continue
		}
		ip, ok := netip.AddrFromSlice(v6[at : at+16])
		if !ok {
			t.Fatal("invalid IPv6 record")
		}
		r := m.Lookup(ip.String())
		if r.Unresolved || !strings.Contains(r.Province, "重庆") {
			t.Fatalf("Chongqing IPv6 record not queryable: %s %+v", ip, r)
		}
		for _, option := range m.Regions() {
			if option.Country == r.Country && option.Province == r.Province {
				t.Logf("IPv6 %s resolves to selectable %s / %s", ip, r.Country, r.Province)
				return
			}
		}
		t.Fatal("queried Chongqing region missing from selection catalog")
	}
	t.Fatal("bundled IPv6 data has no Chongqing region")
}

func TestIPv6FileOverridesBuiltinAndMissingGeoLiteFallsBack(t *testing.T) {
	v6 := loadBundledDualStack(t)
	m := NewManager()
	defer m.Close()
	dir := t.TempDir()
	if err := m.ReloadFromConfig(dir, "ip2region", "geolite2", "opensource", "opensource"); err != nil {
		t.Fatal(err)
	}
	if status := m.GetStatus(); status.IPv6Source != "ip2region" || !status.IPv6Builtin {
		t.Fatalf("missing GeoLite2 did not fall back: %+v", status)
	}
	if err := os.WriteFile(filepath.Join(dir, "ip2region_v6.xdb"), v6, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := m.ReloadFromConfig(dir, "ip2region", "ip2region", "opensource", "opensource"); err != nil {
		t.Fatal(err)
	}
	if m.GetStatus().IPv6Builtin {
		t.Fatal("uploaded IPv6 data must take precedence")
	}
	if err := os.Remove(filepath.Join(dir, "ip2region_v6.xdb")); err != nil {
		t.Fatal(err)
	}
	if err := m.ReloadFromConfig(dir, "ip2region", "ip2region", "opensource", "opensource"); err != nil {
		t.Fatal(err)
	}
	if !m.GetStatus().IPv6Builtin {
		t.Fatal("removing uploaded data must restore bundled IPv6 data")
	}
}
