// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

// HomeWaf sensitive-word adapter over Cloudflare's BSD-3-Clause matcher.
package multimatch

import (
	"github.com/cloudflare/ahocorasick"
)

type Term struct {
	Word       []rune
	CustomData any
}

// Machine is immutable after BuildByCustom; concurrent searches use the
// library's thread-safe matcher. Only non-empty UTF-8 patterns are inserted.
type Machine struct {
	matcher *ahocorasick.Matcher
	words   []string
	data    map[string]any
}

func (m *Machine) BuildByCustom(words [][]rune, data map[string]any) error {
	m.words = nil
	m.data = make(map[string]any, len(data))
	seen := make(map[string]bool)
	for _, word := range words {
		s := string(word)
		if s != "" && !seen[s] {
			seen[s] = true
			m.words = append(m.words, s)
			m.data[s] = data[s]
		}
	}
	if len(m.words) == 0 {
		m.matcher = nil
		return nil
	}
	m.matcher = ahocorasick.NewStringMatcher(m.words)
	return nil
}

func (m *Machine) MultiPatternSearch(input []rune, firstOnly bool) []*Term {
	if m == nil || m.matcher == nil {
		return nil
	}
	s := string(input)
	ids := m.matcher.MatchThreadSafe([]byte(s))
	// The matcher reports hits while scanning left to right; retain that order.
	if firstOnly && len(ids) > 1 {
		ids = ids[:1]
	}
	result := make([]*Term, 0, len(ids))
	for _, id := range ids {
		word := m.words[id]
		result = append(result, &Term{Word: []rune(word), CustomData: m.data[word]})
	}
	return result
}
