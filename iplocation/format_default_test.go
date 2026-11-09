package iplocation

import (
	"os"
	"testing"
)

func TestOpenSourceDefaults(t *testing.T) {
	m := NewManager()
	s := m.GetStatus()
	if s.IPv4Format != "opensource" || s.IPv6Format != "opensource" || s.IPv6Source != "ip2region" {
		t.Fatalf("wrong defaults: %+v", s)
	}
	for _, format := range []DBFormat{"", FormatOpenSource} {
		r := ParseRegion("United States|California|Mountain View|Google LLC|US", format)
		if r.Country != "United States" || r.Province != "California" || r.City != "Mountain View" || r.ISP != "Google LLC" {
			t.Fatalf("misaligned fields: %+v", r)
		}
	}
	r := ParseRegion("中国|江苏省|南京市|电信", FormatOpenSource)
	if r.Province != "江苏省" || r.City != "南京市" || r.ISP != "电信" {
		t.Fatalf("four-field format: %+v", r)
	}
	legacy := ParseRegion("中国|华东|江苏|南京|电信", FormatLegacy)
	if legacy.Region != "华东" || legacy.Province != "江苏" || legacy.ISP != "电信" {
		t.Fatalf("explicit legacy compatibility lost: %+v", legacy)
	}
}

func TestBundledDatabaseUsesOpenSourceFields(t *testing.T) {
	data, err := os.ReadFile("../cmd/homewaf/exedata/ip2region.xdb")
	if err != nil {
		t.Fatal(err)
	}
	m := NewManager()
	if err := m.LoadV4Ip2Region(data, FormatOpenSource); err != nil {
		t.Fatal(err)
	}
	r := m.Lookup("8.8.8.8")
	if r.Country != "United States" || r.Province != "California" || r.ISP != "Google LLC" {
		t.Fatalf("bundled data must use open-source field order: %+v", r)
	}
}
