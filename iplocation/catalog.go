package iplocation

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/ipipdotnet/ipdb-go"
	"github.com/lionsoul2014/ip2region/binding/golang/xdb"
	"github.com/oschwald/geoip2-golang/v2"
	"github.com/oschwald/maxminddb-golang/v2"
)

type RegionOption struct {
	Country  string `json:"country"`
	Province string `json:"province,omitempty"`
}
type regionCollector map[RegionOption]struct{}

func (c regionCollector) add(country, province string) error {
	country, province = strings.TrimSpace(country), strings.TrimSpace(province)
	if country == "" || country == "0" || country == "内网" || country == "未知" || strings.EqualFold(country, "unknown") {
		return nil
	}
	if province == "0" {
		province = ""
	}
	if !utf8.ValidString(country) || !utf8.ValidString(province) || len(country) > 256 || len(province) > 256 {
		return errors.New("invalid database region label")
	}
	c[RegionOption{country, province}] = struct{}{}
	// The bundled database exposes 23,494 pairs in the supported legacy field
	// layout; a 10k cap would incorrectly reject that valid existing database.
	if len(c) > 100000 {
		return errors.New("region catalog exceeds 100000 entries")
	}
	return nil
}
func (c regionCollector) list() []RegionOption {
	out := make([]RegionOption, 0, len(c))
	for r := range c {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Country == out[j].Country {
			return out[i].Province < out[j].Province
		}
		return out[i].Country < out[j].Country
	})
	return out
}

// Enumerate data records once during load, before taking the manager write lock.
// The index layout is the installed xdb library's versioned segment layout.
func xdbRegions(data []byte, version *xdb.Version, format DBFormat) ([]RegionOption, error) {
	header, err := xdb.LoadHeaderFromBuff(data)
	if err != nil {
		return nil, err
	}
	start, end, size := uint64(header.StartIndexPtr), uint64(header.EndIndexPtr), uint64(version.SegmentIndexSize)
	if start == 0 && end == 0 {
		return []RegionOption{}, nil
	}
	if start < xdb.HeaderInfoLength || end < start || (end-start)%size != 0 || end+size > uint64(len(data)) {
		return nil, errors.New("invalid XDB segment bounds")
	}
	collector := regionCollector{}
	seen := map[uint64]struct{}{}
	for at := start; at <= end; at += size {
		offset := at + uint64(2*version.Bytes)
		length := uint64(binary.LittleEndian.Uint16(data[offset:]))
		ptr := uint64(binary.LittleEndian.Uint32(data[offset+2:]))
		if ptr+length > uint64(len(data)) {
			return nil, errors.New("invalid XDB region pointer")
		}
		if length == 0 {
			continue
		}
		key := ptr<<16 | length
		if _, ok := seen[key]; ok {
			continue
		}
		if len(seen) < 100000 {
			seen[key] = struct{}{}
		}
		region := ParseRegion(string(data[ptr:ptr+length]), format)
		if err := collector.add(region.Country, region.Province); err != nil {
			return nil, err
		}
	}
	return collector.list(), nil
}
func mmdbRegions(data []byte) ([]RegionOption, error) {
	db, err := maxminddb.OpenBytes(data)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	collector := regionCollector{}
	for result := range db.Networks() {
		var record geoip2.Country
		if err := result.Decode(&record); err != nil {
			return nil, err
		}
		country := record.Country.Names.SimplifiedChinese
		if country == "" {
			country = record.Country.Names.English
		}
		if err := collector.add(country, ""); err != nil {
			return nil, err
		}
	}
	return collector.list(), nil
}
func ipdbRegions(data []byte) ([]RegionOption, error) {
	if len(data) < 4 {
		return nil, errors.New("short IPDB")
	}
	metaLen := uint64(binary.BigEndian.Uint32(data))
	if metaLen+4 > uint64(len(data)) {
		return nil, errors.New("invalid IPDB metadata")
	}
	var meta ipdb.MetaData
	if err := json.Unmarshal(data[4:4+metaLen], &meta); err != nil {
		return nil, err
	}
	body := data[4+metaLen:]
	if meta.NodeCount < 0 || meta.NodeCount > len(body)/8 || meta.TotalSize != len(body) {
		return nil, errors.New("invalid IPDB node count")
	}
	lang, ok := meta.Languages["CN"]
	if !ok || lang < 0 {
		return nil, errors.New("IPDB has no supported CN region labels")
	}
	countryAt, provinceAt := -1, -1
	for i, field := range meta.Fields {
		if field == "country_name" {
			countryAt = lang + i
		}
		if field == "region_name" {
			provinceAt = lang + i
		}
	}
	if countryAt < 0 {
		return nil, errors.New("IPDB has no country_name")
	}
	collector := regionCollector{}
	seen := map[uint32]struct{}{}
	for i := 0; i < meta.NodeCount*2; i++ {
		node := binary.BigEndian.Uint32(body[i*4:])
		if uint64(node) <= uint64(meta.NodeCount) {
			continue
		}
		if _, ok := seen[node]; ok {
			continue
		}
		if len(seen) < 100000 {
			seen[node] = struct{}{}
		}
		ptr := uint64(node) - uint64(meta.NodeCount) + uint64(meta.NodeCount)*8
		if ptr+2 > uint64(len(body)) {
			return nil, errors.New("invalid IPDB data pointer")
		}
		length := uint64(binary.BigEndian.Uint16(body[ptr:]))
		if ptr+2+length > uint64(len(body)) {
			return nil, errors.New("invalid IPDB data length")
		}
		values := strings.Split(string(body[ptr+2:ptr+2+length]), "\t")
		if lang+len(meta.Fields) > len(values) {
			return nil, errors.New("invalid IPDB language fields")
		}
		province := ""
		if provinceAt >= 0 {
			province = values[provinceAt]
		}
		if err := collector.add(values[countryAt], province); err != nil {
			return nil, err
		}
	}
	return collector.list(), nil
}

// Regions returns actual values from the currently selected local databases,
// including regions never seen in traffic. No network access or request lookup.
func (m *Manager) Regions() []RegionOption {
	m.mu.RLock()
	defer m.mu.RUnlock()
	collector := regionCollector{}
	add := func(rows []RegionOption) {
		for _, r := range rows {
			collector[r] = struct{}{}
		}
	}
	if m.v4Source == SourceIpdb || m.v6Source == SourceIpdb {
		add(m.ipdbRegions)
	}
	if m.v4Source != SourceIpdb {
		add(m.v4Regions)
	}
	if m.v6Source != SourceIpdb {
		add(m.v6Regions)
	}
	return collector.list()
}

func (m *Manager) updateRegionFormat(v4 bool, format DBFormat) {
	m.mu.RLock()
	data := m.v6RegionData
	if v4 {
		data = m.v4RegionData
	}
	m.mu.RUnlock()
	version := xdb.IPv6
	if v4 {
		version = xdb.IPv4
	}
	var rows []RegionOption
	if len(data) > 0 {
		var err error
		rows, err = xdbRegions(data, version, format)
		if err != nil {
			return
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	current := m.v6RegionData
	if v4 {
		current = m.v4RegionData
	}
	if len(current) != len(data) || len(data) > 0 && &current[0] != &data[0] {
		return
	} // A newer database publication wins.
	if v4 {
		m.v4Format = format
		if m.v4Source == SourceIp2Region {
			m.v4Regions = rows
		}
	} else {
		m.v6Format = format
		if m.v6Source == SourceIp2Region {
			m.v6Regions = rows
		}
	}
	m.generation.Add(1)
}
