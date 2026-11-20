package wafpipeline

import (
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestURLPolicyCanonicalDenialsDoNotExpandAllows(t *testing.T) {
	for _, tc := range []struct {
		name    string
		rule    URLRule
		path    string
		want    Action
		allow   bool
		observe bool
	}{
		{"deny double encoding", URLRule{Match: "exact", Value: "/private", Action: Deny}, "/%2570rivate", Deny, false, false},
		{"deny dot segment", URLRule{Match: "prefix", Value: "/private", Action: Deny}, "/x/../private/file", Deny, false, false},
		{"deny path parameter", URLRule{Match: "exact", Value: "/private", Action: Deny}, "/private;ignored", Deny, false, false},
		{"deny suffix", URLRule{Match: "suffix", Value: "/private", Action: Deny}, "/%2570rivate", Deny, false, false},
		{"deny contains", URLRule{Match: "contains", Value: "/private", Action: Deny}, "/%2570rivate/file", Deny, false, false},
		{"preserve exact query", URLRule{Match: "exact", Value: "/private?raw=%2f", Action: Deny}, "/%2570rivate?raw=%2f", Deny, false, false},
		{"do not decode query", URLRule{Match: "exact", Value: "/private?raw=/", Action: Deny}, "/%2570rivate?raw=%2f", Continue, false, false},
		{"preserve raw allow", URLRule{Match: "prefix", Value: "/private", Action: Allow}, "/private/file", Continue, true, false},
		{"do not expand allow", URLRule{Match: "prefix", Value: "/private", Action: Allow}, "/%2570rivate/file", Continue, false, false},
		{"observe canonical denial", URLRule{Match: "exact", Value: "/private", Action: Deny, Mode: ModeObserve}, "/%2570rivate", Continue, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rule := tc.rule
			rule.ID = "test"
			if rule.Mode == "" {
				rule.Mode = ModeBlock
			}
			s := &Snapshot{Config: PortConfig{Policy: DefaultPolicy()}, Compiled: &CompiledPolicy{Extensions: &compiledExtensions{}}}
			s.Config.Policy.RuleSet.URLs = []URLRule{rule}
			c := NewRequestContext(httptest.NewRequest("GET", "http://test"+tc.path, nil), "port:5000", netip.MustParseAddr("198.51.100.1"), DefaultViewLimits())
			v, err := (&Runtime{}).inspectExtensions(s, c)
			if err != nil || v.Action != tc.want || skipAttack(c, "sqli") != tc.allow {
				t.Fatalf("unexpected URL policy result: %+v %v", v, err)
			}
			if tc.observe && (len(c.Hits) != 1 || !c.Hits[0].Observe) {
				t.Fatal("canonical observation was lost or enforced")
			}
		})
	}
}
