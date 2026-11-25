package iplocation

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	randv2 "math/rand/v2"
	"net/netip"
	"sort"
	"strings"

	"github.com/lionsoul2014/ip2region/binding/golang/xdb"
)

type CityCoverage struct {
	Country      string   `json:"country"`
	Province     string   `json:"province"`
	City         string   `json:"city"`
	IPv4         int      `json:"ipv4"`
	IPv6         int      `json:"ipv6"`
	IPv4Target   int      `json:"ipv4_target"`
	IPv6Target   int      `json:"ipv6_target"`
	IPv4Carriers []string `json:"ipv4_carriers"`
	IPv6Carriers []string `json:"ipv6_carriers"`
}

func cityName(value string) string {
	value = strings.TrimSpace(value)
	if len([]rune(value)) > 2 {
		return strings.TrimSuffix(value, "市")
	}
	return value
}
func mainlandCountry(value string) bool {
	return value == "中国" || strings.EqualFold(value, "China") || strings.EqualFold(value, "CN")
}
func carrierName(value string) string {
	lower := strings.ToLower(value)
	for _, c := range []struct {
		name string
		keys []string
	}{{"移动", []string{"移动", "china mobile"}}, {"联通", []string{"联通", "unicom"}}, {"电信", []string{"电信", "telecom"}}, {"广电", []string{"广电", "广播电视", "broadnet"}}} {
		for _, key := range c.keys {
			if strings.Contains(lower, key) {
				return c.name
			}
		}
	}
	if strings.TrimSpace(value) == "" {
		return "运营商未知"
	}
	return strings.TrimSpace(value)
}

// SampleCities covers every named Chinese city in the active datasets.
// Five means five distinct addresses, not five different carrier companies.
func (m *Manager) SampleCities(ctx context.Context, v4Enabled, v6Enabled bool) ([]SampleAddress, []CityCoverage, error) {
	generation := m.Generation()
	m.mu.RLock()
	data4, data6, format4, format6, source4, source6 := m.v4RegionData, m.v6RegionData, m.v4Format, m.v6Format, m.v4Source, m.v6Source
	m.mu.RUnlock()
	var seed [16]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return nil, nil, err
	}
	random := randv2.New(randv2.NewPCG(binary.LittleEndian.Uint64(seed[:8]), binary.LittleEndian.Uint64(seed[8:])))
	type cityPool struct {
		province, city string
		families       [2]map[string][]sampleSegment
	}
	cities := map[string]*cityPool{}
	outside := [2][]sampleSegment{}
	seen := map[string]int{}
	for family := 0; family < 2; family++ {
		data, format, source, version := data4, format4, source4, xdb.IPv4
		if family == 1 {
			data, format, source, version = data6, format6, source6, xdb.IPv6
		}
		if source != SourceIp2Region || len(data) == 0 {
			continue
		}
		header, err := xdb.LoadHeaderFromBuff(data)
		if err != nil {
			return nil, nil, err
		}
		start, end, size := uint64(header.StartIndexPtr), uint64(header.EndIndexPtr), uint64(version.SegmentIndexSize)
		if start < xdb.HeaderInfoLength || end < start || (end-start)%size != 0 || end+size > uint64(len(data)) {
			return nil, nil, errors.New("IP 数据库索引无效")
		}
		cache := map[uint64]*IPLocationResult{}
		for at := start; at <= end; at += size {
			if (at-start)/size%1024 == 0 {
				if err := ctx.Err(); err != nil {
					return nil, nil, err
				}
			}
			offset := at + uint64(2*version.Bytes)
			length, ptr := uint64(binary.LittleEndian.Uint16(data[offset:])), uint64(binary.LittleEndian.Uint32(data[offset+2:]))
			if ptr+length > uint64(len(data)) {
				return nil, nil, errors.New("IP 数据库记录越界")
			}
			recordKey := ptr<<16 | length
			region := cache[recordKey]
			if region == nil {
				region = ParseRegion(string(data[ptr:ptr+length]), format)
				if format == FormatLegacy && region.ISP == "CN" && mainlandCountry(region.Country) {
					return nil, nil, errors.New("IP 数据库字段格式不匹配：国家代码被读作运营商。请在 IP 数据库管理中选择正确格式后重试")
				}
				cache[recordKey] = region
			}
			domestic := mainlandCountry(region.Country)
			city := cityName(region.City)
			if domestic && city == "" {
				for _, municipality := range []string{"北京", "上海", "天津", "重庆"} {
					if cityName(region.Province) == municipality {
						city = municipality
						break
					}
				}
			}
			if domestic && city == "" {
				continue
			}
			if region.Country == "" || strings.EqualFold(region.Country, "reserved") || strings.EqualFold(region.Country, "unknown") || region.Country == "内网" || region.Country == "未知" {
				continue
			}
			read := func(pos uint64) netip.Addr {
				if family == 0 {
					var bytes [4]byte
					binary.BigEndian.PutUint32(bytes[:], binary.LittleEndian.Uint32(data[pos:]))
					return netip.AddrFrom4(bytes)
				}
				var bytes [16]byte
				copy(bytes[:], data[pos:pos+16])
				return netip.AddrFrom16(bytes)
			}
			segment := sampleSegment{read(at), read(at + uint64(version.Bytes)), region}
			if !samplePublic(segment.start) || !samplePublic(segment.end) {
				continue
			}
			if !domestic {
				key := "outside-" + version.Name
				seen[key]++
				if len(outside[family]) < 96 {
					outside[family] = append(outside[family], segment)
				} else if n := random.IntN(seen[key]); n < 96 {
					outside[family][n] = segment
				}
				continue
			}
			key := cityName(region.Province) + "|" + city
			pool := cities[key]
			if pool == nil {
				pool = &cityPool{province: region.Province, city: city, families: [2]map[string][]sampleSegment{{}, {}}}
				cities[key] = pool
			}
			operator := carrierName(region.ISP)
			counterKey := version.Name + "|" + key + "|" + operator
			seen[counterKey]++
			candidates := pool.families[family][operator]
			if len(candidates) < 12 {
				candidates = append(candidates, segment)
			} else if n := random.IntN(seen[counterKey]); n < 12 {
				candidates[n] = segment
			}
			pool.families[family][operator] = candidates
		}
	}
	if len(cities) == 0 {
		return nil, nil, errors.New("当前数据库没有可用的中国城市记录。请检查 IPv4 / IPv6 数据库的加载状态与字段格式")
	}
	keys := []string{}
	for key := range cities {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := []SampleAddress{}
	coverage := []CityCoverage{}
	used := map[string]bool{}
	choose := func(pools map[string][]sampleSegment, family, target int) ([]SampleAddress, []string) {
		operators := []string{}
		for operator := range pools {
			operators = append(operators, operator)
		}
		priority := func(value string) int {
			for i, carrier := range []string{"移动", "联通", "电信", "广电"} {
				if value == carrier {
					return i
				}
			}
			return 4
		}
		sort.Slice(operators, func(i, j int) bool {
			a, b := priority(operators[i]), priority(operators[j])
			if a != b {
				return a < b
			}
			return operators[i] < operators[j]
		})
		selected := []SampleAddress{}
		carriers := []string{}
		carrierSeen := map[string]bool{}
		for round := 0; round < target*3 && len(selected) < target; round++ {
			for _, operator := range operators {
				pool := pools[operator]
				if len(pool) == 0 {
					continue
				}
				for attempt := 0; attempt < 12; attempt++ {
					segment := pool[random.IntN(len(pool))]
					address, err := randomSegmentAddress(segment.start, segment.end)
					if err != nil || used[address.String()] {
						continue
					}
					used[address.String()] = true
					r := segment.region
					version := "IPv4"
					if family == 1 {
						version = "IPv6"
					}
					selected = append(selected, SampleAddress{address.String(), r.Country, r.Province, r.City, operator, version})
					if !carrierSeen[operator] {
						carrierSeen[operator] = true
						carriers = append(carriers, operator)
					}
					break
				}
				if len(selected) == target {
					break
				}
			}
		}
		return selected, carriers
	}
	for _, key := range keys {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		pool := cities[key]
		v4Target, v6Target := 5, 5
		if !v4Enabled {
			v4Target = 1
		}
		if !v6Enabled {
			v6Target = 1
		}
		v4, operators4 := choose(pool.families[0], 0, v4Target)
		v6, operators6 := choose(pool.families[1], 1, v6Target)
		out = append(out, v4...)
		out = append(out, v6...)
		coverage = append(coverage, CityCoverage{"中国", pool.province, pool.city, len(v4), len(v6), v4Target, v6Target, operators4, operators6})
	}
	for family := 0; family < 2; family++ {
		pools := map[string][]sampleSegment{}
		for _, segment := range outside[family] {
			key := segment.region.Country + " · " + carrierName(segment.region.ISP)
			pools[key] = append(pools[key], segment)
		}
		target := 5
		if family == 0 && !v4Enabled || family == 1 && !v6Enabled {
			target = 1
		}
		selected, _ := choose(pools, family, target)
		out = append(out, selected...)
	}
	if generation != m.Generation() {
		return nil, nil, errors.New("生成方案期间 IP 数据库已更新，请重新检测")
	}
	for _, sample := range out {
		actual := m.Lookup(sample.IP)
		if actual.Unresolved || actual.Country != sample.Country || actual.Province != sample.Province || actual.City != sample.City {
			return nil, nil, errors.New("城市取样与实际数据库查询不一致，请重新加载数据库")
		}
	}
	return out, coverage, nil
}
