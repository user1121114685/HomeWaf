package ipset

import (
	"net"
	"testing"
)

func TestTimedPrefixOverlapExpiryAndMappedAddress(t *testing.T) {
	m, err := BuildTimedSet([]TimedEntry{{"203.0.113.0/24", 100}, {"203.0.113.8/32", 200}, {"2001:db8::/64", 300}})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		ip   string
		now  int64
		want bool
	}{{"203.0.113.9", 99, true}, {"203.0.113.9", 100, false}, {"::ffff:203.0.113.8", 150, true}, {"203.0.113.8", 200, false}, {"2001:db8::9", 299, true}, {"2001:db8::9", 300, false}} {
		if got := m.Contains(net.ParseIP(c.ip), c.now); got != c.want {
			t.Fatalf("%s at %d = %v", c.ip, c.now, got)
		}
	}
}

func TestPermanentFastPathAndMixedExpiry(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		entries := []TimedEntry{{"203.0.113.0/24", 0}, {"2001:db8::/64", 0}, {"198.*.7.*", 0}}
		if mixed {
			entries = append(entries, TimedEntry{"192.0.2.0/24", 100})
		}
		m, err := BuildTimedSet(entries)
		if err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			ip   string
			now  int64
			want bool
		}{{"::ffff:203.0.113.9", 9999, true}, {"2001:db8::9", 9999, true}, {"198.51.7.9", 9999, true}, {"203.0.114.9", 50, false}, {"2001:db8:1::9", 50, false}, {"192.0.2.9", 99, mixed}, {"192.0.2.9", 100, false}, {"invalid", 50, false}} {
			if got := m.Contains(net.ParseIP(tc.ip), tc.now); got != tc.want {
				t.Fatalf("mixed=%v ip=%s at=%d got=%v want=%v", mixed, tc.ip, tc.now, got, tc.want)
			}
		}
	}
	for _, m := range []*TimedSet{nil, {}} {
		if m.Contains(net.ParseIP("203.0.113.1"), 50) || m.Contains(nil, 50) {
			t.Fatal("nil or empty index matched")
		}
	}
}
