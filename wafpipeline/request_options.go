package wafpipeline

import (
	"HomeWaf/model"
	"HomeWaf/wafenginecore/antileech"
	"errors"
	"net/url"
	"strings"
)

type AntiLeechPolicy struct {
	Mode     Mode                  `json:"mode"`
	Settings model.AntiLeechConfig `json:"settings"`
}
type ChallengePolicy struct {
	Mode  Mode  `json:"mode,omitempty"`
	Until int64 `json:"until,omitempty"`
	Scope Scope `json:"scope"`
}

func splitOptionList(value string) []string {
	var out []string
	for _, s := range strings.FieldsFunc(value, func(r rune) bool { return r == '\n' || r == '\r' || r == ';' || r == ',' }) {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
func validateRequestOptions(r RuleSetConfig) error {
	if err := validatePluginPolicy(r.Plugins); err != nil {
		return err
	}
	if err := validateCallback(r.Callback); err != nil {
		return err
	}
	if _, err := compileSensitive(r.Sensitive); err != nil {
		return err
	}
	if c := r.UploadOptions; c != nil {
		if c.MaxSizeKB < 0 || c.MaxSizeKB > 1<<20 || len(c.ExtBlacklist) > 8192 || len(c.IncludePaths)+len(c.ExcludePaths) > 32768 || c.OverLimitAction != "pass" && c.OverLimitAction != "block" {
			return errors.New("invalid upload options")
		}
	}
	if c := r.CSRFOptions; c != nil {
		if len(c.ProtectMethods) > 256 || len(c.AllowedOrigins)+len(c.ExcludePaths) > 32768 {
			return errors.New("CSRF options quota exceeded")
		}
		for _, origin := range splitOptionList(c.AllowedOrigins) {
			if !strings.Contains(origin, "://") {
				origin = "https://" + origin
			}
			if _, ok := CanonicalOrigin(origin, false); !ok {
				return errors.New("invalid configured CSRF origin")
			}
		}
	}
	if c := r.AntiLeech; c != nil {
		if !ValidMode(c.Mode) || len(c.Settings.FileTypes) > 1024 || len(c.Settings.ValidReferers) > 8192 {
			return errors.New("invalid referer policy")
		}
		if _, err := antileech.Compile(c.Settings.FileTypes, c.Settings.ValidReferers); err != nil {
			return err
		}
		if c.Settings.Action != "" && c.Settings.Action != "block" && c.Settings.Action != "redirect" {
			return errors.New("invalid referer action")
		}
		if c.Settings.Action == "redirect" {
			u, err := url.Parse(c.Settings.RedirectURL)
			if err != nil || len(c.Settings.RedirectURL) > 2048 || strings.ContainsAny(c.Settings.RedirectURL, "\r\n") || u.User != nil || (u.IsAbs() && (u.Scheme != "http" && u.Scheme != "https")) || (!u.IsAbs() && (!strings.HasPrefix(c.Settings.RedirectURL, "/") || strings.HasPrefix(c.Settings.RedirectURL, "//"))) {
				return errors.New("invalid referer redirect")
			}
		}
	}
	if c := r.Challenge; c.Mode != "" && !ValidMode(c.Mode) || c.Until < 0 {
		return errors.New("invalid challenge policy")
	}
	return ValidateScope(r.Challenge.Scope)
}

func antiLeechStep(s *Snapshot) []Step {
	cfg := s.Config.Policy.RuleSet.AntiLeech
	if cfg == nil || cfg.Mode == ModeOff || cfg.Settings.IsEnableAntiLeech != 1 {
		return nil
	}
	m, _ := antileech.Compile(cfg.Settings.FileTypes, cfg.Settings.ValidReferers)
	return []Step{{Number: 3, Name: "anti_leech", Inspect: func(c *RequestContext) (Verdict, error) {
		if skipAttack(c, "anti_leech") || !m.Protected(c.View.Path) {
			return Verdict{Action: Continue}, nil
		}
		names := append([]string{}, s.Config.HostNames...)
		u, _ := url.Parse("http://" + c.Request.Host)
		if u != nil {
			names = append(names, u.Hostname())
		}
		if m.Allowed(c.Request.Referer(), names) {
			return Verdict{Action: Continue}, nil
		}
		observe := cfg.Mode == ModeObserve
		c.Hit("anti_leech", "anti_leech", Low, observe, "referer policy mismatch")
		status := 403
		if !observe && cfg.Settings.Action == "redirect" {
			u, _ := url.Parse(cfg.Settings.RedirectURL)
			if u != nil && !(u.Host == "" || strings.EqualFold(u.Host, c.Request.Host)) || u != nil && u.Path != c.Request.URL.Path {
				c.Values["policy_redirect"] = cfg.Settings.RedirectURL
				status = 302
			}
		}
		return Verdict{Action: Deny, Status: status, Observe: observe, Reason: "anti_leech"}, nil
	}}}
}
