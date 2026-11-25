package iplocation

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"math/big"
	randv2 "math/rand/v2"
	"net/netip"
	"strings"

	"github.com/lionsoul2014/ip2region/binding/golang/xdb"
)

const SamplesPerFamily = 12

type SampleAddress struct {
	IP       string `json:"ip"`
	Country  string `json:"country"`
	Province string `json:"province"`
	City     string `json:"city"`
	ISP      string `json:"isp"`
	Family   string `json:"family"`
}

type sampleSegment struct {
	start, end netip.Addr
	region     *IPLocationResult
}

// SampleAddresses samples the whole active indexes, with an equal family quota.
// Addresses come from inside distinct network ranges, not a list of range starts.
func (m *Manager) SampleAddresses(ctx context.Context, wanted []RegionOption) ([]SampleAddress, error) {
	if len(wanted) > 12 {
		wanted = wanted[:12]
	}
	generation := m.Generation()
	m.mu.RLock()
	data4, data6, format4, format6, source4, source6 := m.v4RegionData, m.v6RegionData, m.v4Format, m.v6Format, m.v4Source, m.v6Source
	m.mu.RUnlock()
	if source4 != SourceIp2Region || len(data4) == 0 {
		return nil, errors.New("自动检测需要 IPv4 地区库。请在 IP 数据库管理中启用 ip2region 数据库后重试")
	}
	if source6 != SourceIp2Region || len(data6) == 0 {
		return nil, errors.New("自动检测需要 IPv6 地区库，当前无法完成各占 50% 的双栈检测。请启用 IPv6 ip2region 数据库后重试")
	}
	var seed [16]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return nil, err
	}
	random := randv2.New(randv2.NewPCG(binary.LittleEndian.Uint64(seed[:8]), binary.LittleEndian.Uint64(seed[8:])))
	out := []SampleAddress{}
	for _, v4 := range []bool{true, false} {
		data, format, version := data6, format6, xdb.IPv6
		if v4 {
			data, format, version = data4, format4, xdb.IPv4
		}
		header, err := xdb.LoadHeaderFromBuff(data)
		if err != nil {
			return nil, err
		}
		start, end, size := uint64(header.StartIndexPtr), uint64(header.EndIndexPtr), uint64(version.SegmentIndexSize)
		if start < xdb.HeaderInfoLength || end < start || (end-start)%size != 0 || end+size > uint64(len(data)) {
			return nil, errors.New("IP 数据库索引无效，请重新加载数据库")
		}
		pools := make([][]sampleSegment, len(wanted)+2)
		counts := make([]int, len(pools))
		cache := map[uint64]*IPLocationResult{}
		for at := start; at <= end; at += size {
			if (at-start)/size%1024 == 0 {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
			}
			offset := at + uint64(2*version.Bytes)
			length, ptr := uint64(binary.LittleEndian.Uint16(data[offset:])), uint64(binary.LittleEndian.Uint32(data[offset+2:]))
			if ptr+length > uint64(len(data)) {
				return nil, errors.New("IP 数据库记录越界")
			}
			key := ptr<<16 | length
			region := cache[key]
			if region == nil {
				region = ParseRegion(string(data[ptr:ptr+length]), format)
				cache[key] = region
			}
			if region.Country == "" || region.Country == "未知" || region.Country == "内网" || strings.EqualFold(region.Country, "reserved") || strings.EqualFold(region.Country, "unknown") {
				continue
			}
			bucket := len(wanted) + 1
			for _, r := range wanted {
				if strings.EqualFold(r.Country, region.Country) {
					bucket = len(wanted)
					break
				}
			}
			for i, r := range wanted {
				if strings.EqualFold(r.Country, region.Country) && (r.Province == "" || strings.EqualFold(r.Province, region.Province)) {
					bucket = i
					break
				}
			}
			counts[bucket]++
			index := len(pools[bucket])
			if index >= 96 {
				index = random.IntN(counts[bucket])
				if index >= 96 {
					continue
				}
			}
			readIP := func(at uint64) netip.Addr {
				if v4 {
					var b [4]byte
					binary.BigEndian.PutUint32(b[:], binary.LittleEndian.Uint32(data[at:]))
					return netip.AddrFrom4(b)
				}
				var b [16]byte
				copy(b[:], data[at:at+16])
				return netip.AddrFrom16(b)
			}
			segment := sampleSegment{readIP(at), readIP(at + uint64(version.Bytes)), region}
			if !samplePublic(segment.start) || !samplePublic(segment.end) {
				continue
			}
			if index == len(pools[bucket]) {
				pools[bucket] = append(pools[bucket], segment)
			} else {
				pools[bucket][index] = segment
			}
		}
		for _, pool := range pools {
			random.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
		}
		seen := map[string]bool{}
		diversity := map[string]bool{}
		selected := []SampleAddress{}
		take := func(segment sampleSegment, preferDiversity bool) bool {
			region := segment.region
			key := region.Country + "|" + region.Province + "|" + region.ISP
			if preferDiversity && diversity[key] {
				return false
			}
			address, err := randomSegmentAddress(segment.start, segment.end)
			if err != nil {
				return false
			}
			bits := 48
			if v4 {
				bits = 24
			}
			network := netip.PrefixFrom(address, bits).Masked().String()
			if seen[network] {
				return false
			}
			seen[network] = true
			diversity[key] = true
			selected = append(selected, SampleAddress{address.String(), region.Country, region.Province, region.City, region.ISP, version.Name})
			return true
		}
		cursors := make([]int, len(pools))
		for _, diverse := range []bool{true, false} {
			for i := range cursors {
				cursors[i] = 0
			}
			for len(selected) < SamplesPerFamily {
				changed := false
				for bucket, pool := range pools {
					for cursors[bucket] < len(pool) {
						s := pool[cursors[bucket]]
						cursors[bucket]++
						if take(s, diverse) {
							changed = true
							break
						}
					}
					if len(selected) == SamplesPerFamily {
						break
					}
				}
				if !changed {
					break
				}
			}
		}
		if len(selected) != SamplesPerFamily {
			return nil, errors.New(version.Name + " 数据库中可用网段不足，无法完成均衡检测；请更新数据库后重试")
		}
		out = append(out, selected...)
	}
	if m.Generation() != generation {
		return nil, errors.New("IP 数据库已更新，请重新开始检测")
	}
	for _, sample := range out {
		actual := m.Lookup(sample.IP)
		if actual.Unresolved || actual.Country != sample.Country || actual.Province != sample.Province {
			return nil, errors.New("取样归属地与当前数据库不一致，请重新加载后检测")
		}
	}
	return out, nil
}

func randomSegmentAddress(start, end netip.Addr) (netip.Addr, error) {
	width := 16
	if start.Is4() {
		width = 4
	}
	lo, hi := new(big.Int).SetBytes(start.AsSlice()), new(big.Int).SetBytes(end.AsSlice())
	size := new(big.Int).Sub(hi, lo)
	size.Add(size, big.NewInt(1))
	if size.Sign() <= 0 {
		return netip.Addr{}, errors.New("invalid address range")
	}
	for range 32 {
		offset, err := rand.Int(rand.Reader, size)
		if err != nil {
			return netip.Addr{}, err
		}
		value := new(big.Int).Add(lo, offset)
		bytes := make([]byte, width)
		value.FillBytes(bytes)
		if bytes[width-1] < 2 || bytes[width-1] == 255 {
			continue
		}
		address, ok := netip.AddrFromSlice(bytes)
		if ok && samplePublic(address) {
			return address, nil
		}
	}
	return netip.Addr{}, errors.New("range has no suitable host address")
}

var sampleReserved = func() []netip.Prefix {
	out := []netip.Prefix{}
	for _, raw := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32"} {
		out = append(out, netip.MustParsePrefix(raw))
	}
	return out
}()

func samplePublic(ip netip.Addr) bool {
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() {
		return false
	}
	for _, prefix := range sampleReserved {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}
