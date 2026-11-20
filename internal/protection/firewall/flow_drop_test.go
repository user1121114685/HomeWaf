package firewall

import (
	"errors"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func TestFlowDropUsesExactConnectionAndReconcilesOnlyExpiredOwnedRules(t *testing.T) {
	f := NewFlowDropper()
	f.os = "linux"
	now := time.Unix(200000, 0)
	f.now = func() time.Time { return now }
	var commands []string
	f.command = func(bin string, args ...string) (string, error) {
		commands = append(commands, bin+" "+strings.Join(args, " "))
		return "", nil
	}
	if err := f.Drop(netip.MustParseAddrPort("192.0.2.10:5000"), netip.MustParseAddrPort("198.51.100.20:45678")); err != nil {
		t.Fatal(err)
	}
	if len(commands) != 2 || !strings.Contains(commands[0], "-I OUTPUT 1 -p tcp -s 192.0.2.10 -d 198.51.100.20 --sport 5000 --dport 45678") || !strings.Contains(commands[1], "-I INPUT 1 -p tcp -s 198.51.100.20 -d 192.0.2.10 --sport 45678 --dport 5000") {
		t.Fatal(commands)
	}
	commands = nil
	if err := f.Drop(netip.MustParseAddrPort("[2001:db8::1]:5000"), netip.MustParseAddrPort("[2001:db8::2]:45678")); err != nil {
		t.Fatal(err)
	}
	if len(commands) != 2 || !strings.HasPrefix(commands[0], "ip6tables ") || !strings.Contains(commands[1], "-s 2001:db8::2 -d 2001:db8::1 --sport 45678 --dport 5000") {
		t.Fatal(commands)
	}
	commands = nil
	f.command = func(bin string, args ...string) (string, error) {
		line := strings.Join(args, " ")
		commands = append(commands, line)
		if bin == "iptables" && line == "-w 1 -S INPUT" {
			return "-A INPUT -p tcp --dport 5000 -m comment --comment HomeWafDefense_199999_abc -j DROP\n-A INPUT -p tcp --dport 5000 -m comment --comment HomeWafDefense_200100_abc -j DROP\n-A INPUT -s 203.0.113.1 -j DROP", nil
		}
		return "", nil
	}
	if err := f.Reconcile(); err != nil {
		t.Fatal(err)
	}
	deletes := 0
	for _, line := range commands {
		if strings.Contains(line, " -D ") {
			deletes++
			if !strings.Contains(line, "HomeWafDefense_199999_abc") {
				t.Fatal(line)
			}
		}
	}
	if deletes != 1 {
		t.Fatal(commands)
	}
}

func TestFlowDropReportsFailureAndUnsupportedPlatform(t *testing.T) {
	f := NewFlowDropper()
	f.os = "linux"
	f.command = func(bin string, args ...string) (string, error) { return "", errors.New("permission denied") }
	if f.Probe("ipv4") == nil || f.Drop(netip.MustParseAddrPort("[2001:db8::1]:5000"), netip.MustParseAddrPort("[2001:db8::2]:44444")) == nil || f.Error() == "" {
		t.Fatal("failed firewall operation reported success")
	}
	f.os = "windows"
	if f.Probe("ipv4") == nil {
		t.Fatal("unsupported silent drop advertised")
	}
}
