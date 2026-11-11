package iplocation

import (
	"encoding/binary"
	"os"
	"reflect"
	"testing"
)

func TestRegionCatalogEnumeratesUnseenRecordsAndInvalidatesOnFormatChange(t *testing.T) {
	data, err := os.ReadFile(builtinV4Path)
	if err != nil {
		t.Fatal(err)
	}
	m := NewManager()
	defer m.Close()
	if err := m.LoadV4Ip2Region(data, FormatOpenSource); err != nil {
		t.Fatal(err)
	}
	regions := m.Regions()
	if len(regions) < 200 {
		t.Fatalf("catalog depends on observed traffic: %d", len(regions))
	}
	r := m.Lookup("8.8.8.8")
	found := false
	for _, entry := range regions {
		if entry.Country == r.Country && entry.Province == r.Province {
			found = true
		}
	}
	if !found {
		t.Fatal("catalog labels disagree with lookup")
	}
	generation := m.Generation()
	m.SetV4Format(FormatLegacy)
	if m.Generation() == generation || len(m.Regions()) == len(regions) {
		t.Fatal("format change did not rebuild actual region labels")
	}
	m.SetV4Format(FormatOpenSource)
	if !reflect.DeepEqual(m.Regions(), regions) {
		t.Fatal("format round trip changed catalog")
	}
	bad := append([]byte(nil), data...)
	start := binary.LittleEndian.Uint32(bad[8:])
	binary.LittleEndian.PutUint32(bad[int(start)+10:], ^uint32(0))
	generation = m.Generation()
	if err := m.LoadV4Ip2Region(bad, FormatOpenSource); err == nil {
		t.Fatal("invalid region pointer accepted")
	}
	if m.Generation() != generation || !reflect.DeepEqual(m.Regions(), regions) {
		t.Fatal("bad database replaced healthy catalog")
	}
	t.Logf("actual database: %d country/province choices before any request", len(regions))
}
