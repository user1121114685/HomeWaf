package ipset

import (
	"fmt"
	"net"
	"testing"
)

var timedBenchmarkResult bool

// Compare identical permanent prefixes and addresses; TTL lookup is a separate
// feature benchmark and must not be reported as a regression of the old index.
func BenchmarkPermanentIndex100kComparison(b *testing.B) {
	for _, family := range []string{"v4", "v6"} {
		entries := make([]TimedEntry, 100000)
		for i := range entries {
			value := fmt.Sprintf("%d.%d.%d.0/24", 10+(i>>16), (i>>8)&255, i&255)
			if family == "v6" {
				value = fmt.Sprintf("2001:db8:%x:%x::/64", i>>16, i&65535)
			}
			entries[i] = TimedEntry{Value: value}
		}
		index, err := BuildTimedSet(entries)
		if err != nil {
			b.Fatal(err)
		}
		hit, miss := "11.134.159.42", "11.134.160.42"
		if family == "v6" {
			hit, miss = "2001:db8:1:869f::1", "2001:db8:1:86a0::1"
		}
		for _, tc := range []struct {
			name, address string
			want          bool
		}{{"deep-hit", hit, true}, {"adjacent-miss", miss, false}} {
			ip := net.ParseIP(tc.address)
			if index.Contains(ip, 1700000000) != tc.want || index.permanent.Contains(ip) != tc.want {
				b.Fatal("incorrect comparison fixture")
			}
			b.Run(family+"/"+tc.name+"/MatchSet", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					timedBenchmarkResult = index.permanent.Contains(ip)
				}
			})
			b.Run(family+"/"+tc.name+"/TimedSet", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					timedBenchmarkResult = index.Contains(ip, 1700000000)
				}
			})
		}
	}
}

func BenchmarkTimedIndex100kDeep(b *testing.B) {
	const now = int64(1700000000)
	for _, family := range []string{"v4", "v6"} {
		entries := make([]TimedEntry, 100000)
		for i := range entries {
			value := fmt.Sprintf("%d.%d.%d.0/24", 10+(i>>16), (i>>8)&255, i&255)
			if family == "v6" {
				value = fmt.Sprintf("2001:db8:%x:%x::/64", i>>16, i&65535)
			}
			entries[i] = TimedEntry{Value: value, ExpiresAt: now + 3600}
		}
		index, err := BuildTimedSet(entries)
		if err != nil {
			b.Fatal(err)
		}
		hit, miss := "11.134.159.42", "11.134.160.42"
		if family == "v6" {
			hit, miss = "2001:db8:1:869f::1", "2001:db8:1:86a0::1"
		}
		for _, tc := range []struct {
			name, address string
			at            int64
			want          bool
		}{{"deep-hit", hit, now, true}, {"adjacent-miss", miss, now, false}, {"expired", hit, now + 3601, false}} {
			ip := net.ParseIP(tc.address)
			if index.Contains(ip, tc.at) != tc.want {
				b.Fatal("incorrect benchmark fixture")
			}
			b.Run(family+"/"+tc.name, func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					timedBenchmarkResult = index.Contains(ip, tc.at)
				}
			})
		}
	}
}
