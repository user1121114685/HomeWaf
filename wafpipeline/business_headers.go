package wafpipeline

import (
	"HomeWaf/model"
	"encoding/json"
	"errors"
	"golang.org/x/net/http/httpguts"
	"net/http"
	"regexp"
	"strings"
)

// Header values can contain upstream credentials. Only profile references are
// public; complete profiles live in authenticated encrypted PortSecrets.
type BusinessHeaders struct {
	Request  []model.CustomHeaderItem         `json:"request"`
	Response []model.CustomResponseHeaderRule `json:"response"`
}

func CloneSecrets(value PortSecrets) PortSecrets {
	raw, _ := json.Marshal(value)
	var out PortSecrets
	_ = json.Unmarshal(raw, &out)
	return out
}
func permittedBusinessHeader(name string) bool {
	if !httpguts.ValidHeaderFieldName(name) {
		return false
	}
	switch strings.ToLower(name) {
	case "connection", "proxy-connection", "keep-alive", "transfer-encoding", "content-length", "trailer", "te", "upgrade", "proxy-authorization", "proxy-authenticate", "x-homewaf-token", "x-waf-custom-xff-control":
		return false
	}
	return true
}
func ValidateBusinessHeaders(profiles map[string]BusinessHeaders) error {
	if len(profiles) > 200 {
		return errors.New("too many business header profiles")
	}
	total := 0
	check := func(headers []model.CustomHeaderItem) error {
		if len(headers) > 128 {
			return errors.New("too many business headers")
		}
		for _, header := range headers {
			total += len(header.HeaderName) + len(header.HeaderValue)
			if !permittedBusinessHeader(header.HeaderName) || len(header.HeaderValue) > 8192 || !httpguts.ValidHeaderFieldValue(header.HeaderValue) {
				return errors.New("invalid or reserved business header")
			}
		}
		return nil
	}
	for key, profile := range profiles {
		if key == "" || len(key) > 128 || len(profile.Response) > 128 {
			return errors.New("invalid business header profile")
		}
		if err := check(profile.Request); err != nil {
			return err
		}
		for _, rule := range profile.Response {
			if len(rule.MatchValue) > 1024 {
				return errors.New("business header match too long")
			}
			switch rule.MatchType {
			case "", "global", "prefix", "suffix", "exact":
			case "regex":
				if _, err := regexp.Compile(rule.MatchValue); err != nil {
					return errors.New("invalid business header expression")
				}
			default:
				return errors.New("invalid business header match")
			}
			if rule.MergeMode != "" && rule.MergeMode != "merge" && rule.MergeMode != "override" {
				return errors.New("invalid business header merge mode")
			}
			if err := check(rule.Headers); err != nil {
				return err
			}
		}
	}
	if total > 512<<10 {
		return errors.New("business header values exceed quota")
	}
	return nil
}
func applyBusinessHeaders(destination http.Header, headers map[string]string) {
	for name, value := range headers {
		if !permittedBusinessHeader(name) || !httpguts.ValidHeaderFieldValue(value) {
			continue
		}
		if strings.EqualFold(name, "Set-Cookie") && (strings.HasPrefix(strings.ToLower(value), "hw_gate_") || strings.HasPrefix(strings.ToLower(value), "hw_csrf_")) {
			continue
		}
		destination.Set(name, value)
	}
}
func (f *Forwarder) attachHeaders(s *Snapshot, target *ProxyTarget, key string) error {
	if key == "" {
		return nil
	}
	profile, ok := s.Secrets.BusinessHeaders[key]
	if !ok {
		return errors.New("业务头配置不存在，请补充加密头配置")
	}
	if len(profile.Request) > 0 && f.RequestHeaders == nil || len(profile.Response) > 0 && f.ResponseHeaders == nil {
		return errors.New("业务头处理器未就绪")
	}
	if len(profile.Response) > 0 {
		target.responseHeaders = func(r *http.Request, ip string) map[string]string { return f.ResponseHeaders(profile.Response, r, ip) }
	}
	if target.proxy != nil && len(profile.Request) > 0 {
		director := target.proxy.Director
		target.proxy.Director = func(r *http.Request) {
			director(r)
			state, ok := r.Context().Value(forwardingContextKey{}).(*forwardRequest)
			if !ok {
				return
			}
			headers := f.RequestHeaders(profile.Request, state.request.Request, state.request.ClientIP.String())
			applyBusinessHeaders(r.Header, headers)
			if host := r.Header.Get("Host"); host != "" {
				r.Host = host
				r.Header.Del("Host")
			}
		}
	}
	return nil
}
