package ipset

import "net"

type TimedEntry struct {
	Value     string
	ExpiresAt int64
}
type timedNode struct {
	children [2]*timedNode
	expires  int64
}
type timedPattern struct {
	pattern Pattern
	expires int64
}

// TimedSet reuses MatchSet for permanent entries. Only expiring prefixes need
// expiry-aware nodes; expiry checks never rebuild the index on a request.
type TimedSet struct {
	permanent *MatchSet
	v4, v6    *timedNode
	wild      []timedPattern
	hasExpiry bool
}

func BuildTimedSet(entries []TimedEntry) (*TimedSet, error) {
	m := &TimedSet{permanent: BuildMatchSet(nil), v4: &timedNode{}, v6: &timedNode{}}
	for _, e := range entries {
		p, err := ParsePattern(e.Value)
		if err != nil {
			return nil, err
		}
		if e.ExpiresAt == 0 {
			m.permanent.Add(e.Value)
			continue
		}
		m.hasExpiry = true
		root := m.v6
		if p.Width == 4 {
			root = m.v4
		}
		if p.Kind == KindRange {
			for _, prefix := range rangeToPrefixes(p.Value, p.End) {
				insertTimed(root, prefix.Net, prefix.Bits, e.ExpiresAt)
			}
		} else if p.Prefix >= 0 {
			insertTimed(root, p.Value, p.Prefix, e.ExpiresAt)
		} else {
			m.wild = append(m.wild, timedPattern{p, e.ExpiresAt})
		}
	}
	return m, nil
}
func insertTimed(root *timedNode, ip []byte, bits int, expiry int64) {
	n := root
	for i := 0; i < bits; i++ {
		bit := (ip[i/8] >> (7 - uint(i%8))) & 1
		if n.children[bit] == nil {
			n.children[bit] = &timedNode{}
		}
		n = n.children[bit]
	}
	if expiry > n.expires {
		n.expires = expiry
	}
}
func (m *TimedSet) Contains(ip net.IP, now int64) bool {
	if m == nil {
		return false
	}
	// A permanent-only policy uses exactly the existing lookup, including a
	// miss. Do not traverse empty expiry tries after a permanent miss.
	if !m.hasExpiry {
		return m.permanent.Contains(ip)
	}
	return m.containsExpiring(ip, now)
}

// Keep expiry traversal out of the permanent hot path's stack frame.
func (m *TimedSet) containsExpiring(ip net.IP, now int64) bool {
	if ip == nil {
		return false
	}
	if m.permanent.Contains(ip) {
		return true
	}
	root := m.v6
	value := ip.To16()
	if v4 := ip.To4(); v4 != nil {
		root = m.v4
		value = v4
	}
	if value == nil {
		return false
	}
	n := root
	if n.expires > now {
		return true
	}
	for i := 0; i < len(value)*8; i++ {
		bit := (value[i/8] >> (7 - uint(i%8))) & 1
		n = n.children[bit]
		if n == nil {
			break
		}
		if n.expires > now {
			return true
		}
	}
	for _, p := range m.wild {
		if p.expires > now && p.pattern.Match(ip) {
			return true
		}
	}
	return false
}
