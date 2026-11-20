package firewall

import (
	"errors"
	"fmt"
	"net/netip"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"testing"
)

// Tests exercise the real command construction and response parsing without
// changing the host firewall. Unexpected commands fail instead of executing.
func newTestFirewall(t testing.TB) FireWallEngine {
	t.Helper()
	var mu sync.Mutex
	rules := map[string]string{}
	return FireWallEngine{command: func(cmd *exec.Cmd) (error, string) {
		mu.Lock()
		defer mu.Unlock()
		args := cmd.Args
		if len(args) == 0 {
			t.Fatal("empty firewall command")
		}
		if strings.Contains(strings.ToLower(args[0]), "netsh") {
			if len(args) < 5 || args[1] != "advfirewall" || args[2] != "firewall" || args[4] != "rule" {
				t.Fatalf("unexpected netsh command: %v", args)
			}
			options := map[string]string{}
			for _, arg := range args[5:] {
				if k, v, ok := strings.Cut(arg, "="); ok {
					options[k] = v
				}
			}
			name := options["name"]
			switch args[3] {
			case "add":
				ip := options["remoteip"]
				if _, err := netip.ParseAddr(ip); err != nil {
					if _, err := netip.ParsePrefix(ip); err != nil {
						return err, "invalid IP"
					}
				}
				if name == "" || options["dir"] != "in" || options["action"] != "block" {
					t.Fatalf("invalid block command: %v", args)
				}
				rules[name] = ip
				return nil, "Ok."
			case "delete":
				delete(rules, name)
				return nil, "Ok."
			case "show":
				if name != "all" {
					if _, ok := rules[name]; !ok {
						return errors.New("exit status 1"), "No rules match the specified criteria."
					}
				}
				var names []string
				for key := range rules {
					if name == "all" || key == name {
						names = append(names, key)
					}
				}
				sort.Strings(names)
				var lines []string
				for _, key := range names {
					lines = append(lines, "Rule Name: "+key)
				}
				return nil, strings.Join(lines, "\n")
			}
		} else if strings.HasSuffix(args[0], "iptables-save") {
			var lines []string
			for ip := range rules {
				lines = append(lines, "-A INPUT -s "+ip+" -j DROP")
			}
			sort.Strings(lines)
			return nil, strings.Join(lines, "\n")
		} else if strings.HasSuffix(args[0], "iptables") {
			ip := ""
			for i, arg := range args {
				if arg == "-s" && i+1 < len(args) {
					ip = args[i+1]
				}
			}
			if len(args) > 1 && (args[1] == "-I" || args[1] == "-A") {
				rules[ip] = ip
				return nil, ""
			}
			if len(args) > 1 && args[1] == "-D" {
				delete(rules, ip)
				return nil, ""
			}
		}
		t.Fatal(fmt.Sprintf("unexpected firewall command: %v", args))
		return errors.New("unexpected command"), ""
	}}
}
