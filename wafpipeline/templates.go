package wafpipeline

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
)

type Template struct {
	Schema  int    `json:"schema_version"`
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version int    `json:"version"`
	Builtin bool   `json:"builtin"`
	Policy  Policy `json:"policy"`
}

func BuiltinTemplates() []Template {
	names := []struct{ id, name string }{{"family-whitelist", "家庭·仅白名单"}, {"family-nas", "家庭·NAS"}, {"family-device", "家庭·摄像头/智能家居"}, {"public-site", "公开小站点"}, {"api-webhook", "API/Webhook"}, {"observe", "仅观察（学习模式）"}}
	out := make([]Template, 0, len(names))
	for _, n := range names {
		p := DefaultPolicy()
		switch n.id {
		case "family-whitelist":
			p.IPGroup.UnlistedAction = Deny
			p.Gate.Enabled = true
			p.Honeypot.Enabled = true
			p.HitPolicy.Lockdown.Enabled = true
		case "family-nas":
			p.Gate.Enabled = true
			p.RuleSet.QPS.PerIP = 60
			p.RuleSet.QPS.Burst = 200
			for i := range p.RuleSet.Rules {
				if p.RuleSet.Rules[i].Category == "upload" {
					p.RuleSet.Rules[i].Scope = Scope{Paths: []string{"/webdav", "/dav", "/remote.php/dav", "/upload", "/api/upload"}, Methods: []string{"POST", "PUT", "PATCH"}}
				}
			}
		case "family-device":
			p.Gate.Enabled = true
			p.RuleSet.QPS.PerIP = 10
			p.RuleSet.QPS.Burst = 40
			p.Gate.TokenHeader = "X-HomeWaf-Token"
		case "public-site":
			p.RuleSet.CC.Mode = ModeBlock
			p.RuleSet.BOT = ModeBlock
		case "api-webhook":
			p.Gate.TokenHeader = "X-HomeWaf-Token"
			p.Gate.RequireToken = true
			p.Gate.BypassTrusted = false
			p.RuleSet.QPS.Paths = map[string]float64{"/api/": 20, "/webhook/": 10}
			for i := range p.RuleSet.Rules {
				if p.RuleSet.Rules[i].Category == "csrf" {
					p.RuleSet.Rules[i].Mode = ModeOff
				}
			}
		case "observe":
			p.HitPolicy.Enabled = false
			p.Honeypot.Enabled = false
			p.RuleSet.QPS.Mode = ModeObserve
			p.RuleSet.CC.Mode = ModeObserve
			for i := range p.RuleSet.Rules {
				p.RuleSet.Rules[i].Mode = ModeObserve
			}
		}
		out = append(out, Template{Schema: SchemaVersion, ID: n.id, Name: n.name, Version: 1, Builtin: true, Policy: p})
	}
	return out
}
func TemplateFromPort(id, name string, c PortConfig) Template {
	p := CloneConfig(c).Policy
	p.IPGroup.Entries = []IPEntry{}
	p.IPGroup.AllowGroups = nil
	p.IPGroup.DenyGroups = nil
	p.Network.TrustedProxies = []string{}
	p.Gate.ExemptIPs = []string{}
	if p.RuleSet.Callback != nil {
		p.RuleSet.Callback.Networks = nil
	}
	for i := range p.Network.Response.Tamper {
		p.Network.Response.Tamper[i].LearnedAt = 0
	}
	return Template{Schema: SchemaVersion, ID: id, Name: name, Version: 1, Policy: p}
}
func ValidateTemplate(t Template) error {
	if t.Schema != SchemaVersion || t.Version < 1 || t.ID == "" || len(t.ID) > 64 || t.Name == "" || len(t.Name) > 128 {
		return fmt.Errorf("invalid template metadata")
	}
	if len(t.Policy.IPGroup.Entries) != 0 || len(t.Policy.IPGroup.AllowGroups)+len(t.Policy.IPGroup.DenyGroups) != 0 || len(t.Policy.Network.TrustedProxies) != 0 || len(t.Policy.Gate.ExemptIPs) != 0 {
		return fmt.Errorf("templates cannot contain manually entered IP entries or proxy CIDRs")
	}
	if t.Policy.RuleSet.Callback != nil && len(t.Policy.RuleSet.Callback.Networks) > 0 {
		return fmt.Errorf("templates cannot contain callback IP entries")
	}
	data, _ := json.Marshal(t)
	var decoded any
	_ = json.Unmarshal(data, &decoded)
	if hasSecretKey(decoded) {
		return fmt.Errorf("templates cannot contain credentials")
	}
	return ValidatePolicy(t.Policy)
}
func hasSecretKey(v any) bool {
	switch x := v.(type) {
	case map[string]any:
		for k, value := range x {
			n := strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(k))
			for _, forbidden := range []string{"password", "passwordhash", "secret", "secretkey", "privatekey", "token", "tokenhash", "tokenhashes", "serviceTokenHashes", "gatekey", "tlskey", "users", "credentials"} {
				if n == strings.ToLower(forbidden) {
					return true
				}
			}
			if hasSecretKey(value) {
				return true
			}
		}
	case []any:
		for _, value := range x {
			if hasSecretKey(value) {
				return true
			}
		}
	}
	return false
}
func ImportTemplate(data []byte) (Template, error) {
	var t Template
	if err := StrictDecode(data, &t); err != nil {
		return t, err
	}
	t.Builtin = false
	return t, ValidateTemplate(t)
}

var PolicySections = []string{"access", "ip_group", "ruleset", "hit_policy", "honeypot", "gate", "network"}

func ApplyTemplate(c PortConfig, t Template, sections []string) (PortConfig, error) {
	if err := ValidateTemplate(t); err != nil {
		return c, err
	}
	if len(sections) == 0 {
		sections = PolicySections
	}
	out := CloneConfig(c)
	data, _ := json.Marshal(t.Policy)
	var p Policy
	_ = json.Unmarshal(data, &p)
	for _, s := range sections {
		switch s {
		case "access":
			out.Policy.Access = p.Access
		case "ip_group":
			entries := out.Policy.IPGroup.Entries
			allowGroups, denyGroups := out.Policy.IPGroup.AllowGroups, out.Policy.IPGroup.DenyGroups
			out.Policy.IPGroup = p.IPGroup
			out.Policy.IPGroup.Entries = entries
			out.Policy.IPGroup.AllowGroups, out.Policy.IPGroup.DenyGroups = allowGroups, denyGroups
		case "ruleset":
			var networks []string
			if out.Policy.RuleSet.Callback != nil {
				networks = out.Policy.RuleSet.Callback.Networks
			}
			out.Policy.RuleSet = p.RuleSet
			if out.Policy.RuleSet.Callback != nil {
				out.Policy.RuleSet.Callback.Networks = networks
			}
		case "hit_policy":
			out.Policy.HitPolicy = p.HitPolicy
		case "honeypot":
			out.Policy.Honeypot = p.Honeypot
		case "gate":
			ips := out.Policy.Gate.ExemptIPs
			cookiePrefixes, headers := out.Policy.Gate.CredentialCookiePrefixes, out.Policy.Gate.CredentialHeaders
			out.Policy.Gate = p.Gate
			out.Policy.Gate.ExemptIPs = ips
			out.Policy.Gate.CredentialCookiePrefixes = cookiePrefixes
			out.Policy.Gate.CredentialHeaders = headers
		case "network":
			proxies := out.Policy.Network.TrustedProxies
			out.Policy.Network = p.Network
			out.Policy.Network.TrustedProxies = proxies
		default:
			return c, fmt.Errorf("invalid template section %q", s)
		}
	}
	out.Template = TemplateSource{ID: t.ID, Version: t.Version, AppliedAt: time.Now().Unix()}
	return out, nil
}

type SectionDiff struct {
	Section  string          `json:"section"`
	Current  json.RawMessage `json:"current"`
	Template json.RawMessage `json:"template"`
}

func TemplateDiff(c PortConfig, t Template) []SectionDiff {
	clean := TemplateFromPort("diff", "diff", c)
	a, _ := json.Marshal(clean.Policy)
	b, _ := json.Marshal(t.Policy)
	var left, right map[string]json.RawMessage
	_ = json.Unmarshal(a, &left)
	_ = json.Unmarshal(b, &right)
	var out []SectionDiff
	for _, key := range PolicySections {
		if !reflect.DeepEqual(left[key], right[key]) {
			out = append(out, SectionDiff{key, left[key], right[key]})
		}
	}
	return out
}
