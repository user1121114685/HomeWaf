package wafpipeline

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTemplatesCopyIsolationAndRejectCredentials(t *testing.T) {
	for _, tpl := range BuiltinTemplates() {
		if err := ValidateTemplate(tpl); err != nil {
			t.Fatalf("%s: %v", tpl.ID, err)
		}
	}
	c := PortConfig{Schema: 1, Port: 5000, Protocol: "http", Upstream: "http://127.0.0.1:8080", Policy: DefaultPolicy()}
	tpl := BuiltinTemplates()[1]
	out, err := ApplyTemplate(c, tpl, []string{"ruleset"})
	if err != nil {
		t.Fatal(err)
	}
	out.Policy.RuleSet.Rules[0].Mode = ModeOff
	if tpl.Policy.RuleSet.Rules[0].Mode == ModeOff || c.Policy.RuleSet.Rules[0].Mode == ModeOff {
		t.Fatal("template or original aliased")
	}
	raw, _ := json.Marshal(tpl)
	bad := strings.Replace(string(raw), `"policy":{`, `"policy":{"password":"do-not-import",`, 1)
	if _, err := ImportTemplate([]byte(bad)); err == nil {
		t.Fatal("accepted unknown credential")
	}
	tpl.Policy.IPGroup.Entries = []IPEntry{{Type: "allow", Trust: "trusted", Value: "192.0.2.1"}}
	if err := ValidateTemplate(tpl); err == nil {
		t.Fatal("accepted manual IPs in template")
	}
}
func TestPortConfigValidation(t *testing.T) {
	c := PortConfig{Schema: 1, Port: 5000, Protocol: "http", Upstream: "http://127.0.0.1:8080", Policy: DefaultPolicy()}
	if err := ValidateConfig(c, 26666); err != nil {
		t.Fatal(err)
	}
	c.Port = 26666
	if err := ValidateConfig(c, 26666); err == nil {
		t.Fatal("management collision")
	}
	c.Port = 5000
	c.Upstream = "http://user:pass@example.test"
	if err := ValidateConfig(c, 26666); err == nil {
		t.Fatal("upstream credential leak")
	}
}

func TestAllowedHostNamesAcceptIPsWithoutURLOrPort(t *testing.T) {
	for _, host := range []string{"nas.example.test", "*.example.test", "192.0.2.10", "2001:db8::1"} {
		if err := ValidateHostNames([]string{host}); err != nil {
			t.Fatalf("%s: %v", host, err)
		}
	}
	for _, host := range []string{"https://nas.test", "nas.test:443", "nas.test/path", "2001:xyz::1", "fe80::1%eth0", "name\t.test"} {
		if ValidateHostNames([]string{host}) == nil {
			t.Fatalf("accepted %q", host)
		}
	}
}
