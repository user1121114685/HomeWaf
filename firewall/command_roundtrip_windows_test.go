package firewall

import "testing"

func TestNetshRuleNameRoundTripAndMissingStatus(t *testing.T) {
	fw := newTestFirewall(t)
	for _, ip := range []string{"192.0.2.10", "198.51.100.0/24", "2001:db8::1"} {
		if err := fw.BlockIP(ip, "fixture"); err != nil {
			t.Fatal(err)
		}
	}
	list, err := fw.GetBlockedIPList()
	if err != nil || len(list) != 3 {
		t.Fatalf("enumeration=%v %v", list, err)
	}
	for _, ip := range list {
		if yes, err := fw.IsIPBlocked(ip); err != nil || !yes {
			t.Fatalf("decoded wrong rule address %q: %v", ip, err)
		}
	}
	if count, err := fw.ClearAllBlockedIPs(); err != nil || count != 3 {
		t.Fatalf("clear count=%d error=%v", count, err)
	}
	if yes, err := fw.IsRuleExists("absent"); yes || err != nil {
		t.Fatalf("netsh exit1/no-match should mean absent: %v %v", yes, err)
	}
}
