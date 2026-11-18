package wafpipeline

import (
	"encoding/json"
	"fmt"
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

func TestFamilyNASTemplateAllowsMissingCSRFOrigin(t *testing.T) {
	for _, tpl := range BuiltinTemplates() {
		wantMode, wantVersion := ModeObserve, 1
		if tpl.ID == "family-nas" {
			wantMode, wantVersion = ModeOff, 4
		}
		found := false
		for _, rule := range tpl.Policy.RuleSet.Rules {
			if rule.Category == synologyAdminCategory || rule.Category == synologyFailureCategory {
				expected := ModeOff
				if tpl.ID == "family-nas" {
					expected = ModeBlock
				}
				if rule.Mode != expected {
					t.Errorf("%s: %s = %s, want %s", tpl.ID, rule.Category, rule.Mode, expected)
				}
			}
			if rule.Category == "ssrf" {
				found = true
				if rule.Mode != wantMode || tpl.Version != wantVersion {
					t.Fatalf("template %s: SSRF=%s version=%d, want %s/%d", tpl.ID, rule.Mode, tpl.Version, wantMode, wantVersion)
				}
			}
			if tpl.ID == "family-nas" && rule.Category == "csrf" && rule.Mode != ModeObserve {
				t.Fatal("NAS template must retain CSRF checks for supplied origins")
			}
		}
		if tpl.ID == "family-nas" && tpl.Policy.RuleSet.CSRFMissingOrigin != "allow" {
			t.Fatalf("NAS template missing-origin action = %q, want allow", tpl.Policy.RuleSet.CSRFMissingOrigin)
		}
		if !found {
			t.Fatalf("template %s lost the configurable SSRF rule", tpl.ID)
		}
	}
	for _, rule := range DefaultPolicy().RuleSet.Rules {
		if rule.Category == "ssrf" && rule.Mode != ModeObserve {
			t.Fatal("NAS template changed the global SSRF default")
		}
	}
}
func TestLegacySynologyRulesAndValidation(t *testing.T) {
	policy := DefaultPolicy()
	policy.RuleSet.Rules = []Rule{{ID: synologyAdminCategory, Category: "sqli", Mode: ModeObserve, Severity: High}}
	policy.RuleSet.CompleteSynologyRules()
	policy.RuleSet.CompleteSynologyRules()
	if len(policy.RuleSet.Rules) != 3 || policy.RuleSet.Rules[0].Mode != ModeObserve || policy.RuleSet.LocksDSMLogins() {
		t.Fatal("old service settings changed or new rules duplicated")
	}
	if err := ValidatePolicy(policy); err != nil {
		t.Fatal(err)
	}
	failure := policy.RuleSet.ruleByCategory(synologyFailureCategory)
	if failure == nil || failure.LoginFailures == nil || failure.LoginFailures.Count != 10 {
		t.Fatal("missing default DSM window")
	}
	failure.LoginFailures.Count = 0
	if err := ValidatePolicy(policy); err == nil {
		t.Fatal("accepted zero failed logins")
	}
	failure.LoginFailures.Count = 10
	duplicate := *failure
	duplicate.ID = "second-login-window"
	policy.RuleSet.Rules = append(policy.RuleSet.Rules, duplicate)
	if err := ValidatePolicy(policy); err == nil {
		t.Fatal("accepted conflicting DSM windows")
	}
	legacy := DefaultPolicy().RuleSet
	legacy.SynologyLogin = &SynologyLoginPolicy{Admin: ModeBlock, FailureMode: ModeObserve, Failures: Threshold{WindowSeconds: 600, Count: 8, BanSeconds: 3600}}
	legacyRaw := *legacy.SynologyLogin
	legacyFailures := &legacy.SynologyLogin.Failures
	legacy.CompleteSynologyRules()
	admin := legacy.ruleByCategory(synologyAdminCategory)
	failure = legacy.ruleByCategory(synologyFailureCategory)
	if admin.Mode != ModeBlock || failure.Mode != ModeObserve || failure.LoginFailures != legacyFailures || legacy.SynologyLogin != nil {
		t.Fatal("legacy DSM settings were not migrated to detector rules")
	}
	valid := DefaultPolicy()
	valid.RuleSet = legacy
	if err := ValidatePolicy(valid); err != nil {
		t.Fatal(err)
	}
	rawLegacy, err := json.Marshal(RuleSetConfig{SynologyLogin: &legacyRaw})
	if err != nil {
		t.Fatal(err)
	}
	var decoded RuleSetConfig
	if err := StrictDecode(rawLegacy, &decoded); err != nil {
		t.Fatal(err)
	}
	decoded.CompleteSynologyRules()
	if decoded.LocksDSMLogins() || decoded.SynologyLogin != nil || decoded.ruleByCategory(synologyAdminCategory).Mode != ModeBlock || decoded.ruleByCategory(synologyFailureCategory).Mode != ModeObserve {
		t.Fatal("decoded legacy DSM settings were not preserved")
	}
	full := DefaultPolicy()
	full.RuleSet.Rules = make([]Rule, 256)
	for i := range full.RuleSet.Rules {
		full.RuleSet.Rules[i] = Rule{ID: fmt.Sprintf("custom-%d", i), Category: "sqli", Severity: Medium, Mode: ModeObserve}
	}
	full.RuleSet.CompleteSynologyRules()
	if len(full.RuleSet.Rules) != 258 || ValidatePolicy(full) != nil {
		t.Fatal("new DSM rules must not consume the previous 256-rule allowance")
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
