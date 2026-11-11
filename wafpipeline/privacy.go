package wafpipeline

import (
	dlp "HomeWaf/internal/thirdparty/godlp"
	"HomeWaf/internal/thirdparty/godlp/dlpheader"
	"errors"
	"strings"
)

type PrivacyURL struct {
	Match string `json:"match"`
	Value string `json:"value"`
}
type PrivacyPolicy struct {
	Mode Mode         `json:"mode"`
	URLs []PrivacyURL `json:"urls"`
}
type compiledPrivacy struct{ engine dlpheader.EngineAPI }

func validatePrivacyPolicy(p *PrivacyPolicy) error {
	if p == nil {
		return nil
	}
	if !ValidMode(p.Mode) || len(p.URLs) > 256 {
		return errors.New("invalid privacy policy")
	}
	for _, r := range p.URLs {
		if r.Value == "" || len(r.Value) > 2048 {
			return errors.New("invalid privacy URL")
		}
		switch r.Match {
		case "exact", "prefix", "suffix", "contains":
		default:
			return errors.New("invalid privacy URL comparison")
		}
	}
	return nil
}
func compilePrivacy(p *PrivacyPolicy, config string) (*compiledPrivacy, error) {
	if len(config) > 256<<10 {
		return nil, errors.New("privacy engine configuration exceeds quota")
	}
	if p == nil || p.Mode == ModeOff || len(p.URLs) == 0 {
		return nil, nil
	}
	engine, err := dlp.NewEngine("port-privacy")
	if err != nil {
		return nil, errors.New("privacy engine unavailable")
	}
	if config == "" {
		err = engine.ApplyConfigDefault()
	} else {
		err = engine.ApplyConfig(config)
	}
	if err != nil {
		return nil, errors.New("privacy engine configuration invalid")
	}
	return &compiledPrivacy{engine: engine}, nil
}
func privacyResponseActive(s *Snapshot, c *RequestContext) bool {
	p := s.Config.Policy.Network.Response.Privacy
	if p == nil || p.Mode == ModeOff || s.Compiled.Privacy == nil {
		return false
	}
	value := strings.ToLower(c.Request.URL.RequestURI())
	for _, rule := range p.URLs {
		pattern := strings.ToLower(rule.Value)
		switch rule.Match {
		case "exact":
			if value == pattern {
				return true
			}
		case "prefix":
			if strings.HasPrefix(value, pattern) {
				return true
			}
		case "suffix":
			if strings.HasSuffix(value, pattern) {
				return true
			}
		case "contains":
			if strings.Contains(value, pattern) {
				return true
			}
		}
	}
	return false
}
func privacyResponse(s *Snapshot, c *RequestContext, body []byte) (out []byte) {
	out = body
	defer func() {
		if recover() != nil {
			out = body
			c.Hit("privacy_error", "response_privacy", Low, true, "privacy transform unavailable")
		}
	}()
	masked, results, err := s.Compiled.Privacy.engine.Deidentify(string(body))
	if err != nil || len(masked) > 1<<20 || (len(body) > 0 && masked == "") {
		c.Hit("privacy_error", "response_privacy", Low, true, "privacy transform unavailable")
		return body
	}
	if len(results) > 0 {
		c.Hit("privacy_match", "response_privacy", Low, true, "response privacy policy matched")
	}
	if s.Config.Policy.Network.Response.Privacy.Mode == ModeObserve {
		return body
	}
	return []byte(masked)
}
