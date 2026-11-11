package wafpipeline

import (
	"HomeWaf/model"
	"bytes"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type ResponsePolicy struct {
	Privacy         *PrivacyPolicy               `json:"privacy,omitempty"`
	Tamper          []TamperRule                 `json:"tamper,omitempty"`
	Cookie          model.CookieSecurityConfig   `json:"cookie"`
	Compression     model.ResponseCompressConfig `json:"compression"`
	DefaultEncoding string                       `json:"default_encoding,omitempty"`
}

func validateResponsePolicy(p ResponsePolicy) error {
	if err := validatePrivacyPolicy(p.Privacy); err != nil {
		return err
	}
	if err := validateTamperRules(p.Tamper); err != nil {
		return err
	}
	if p.Cookie.HttpOnly < 0 || p.Cookie.HttpOnly > 1 || p.Cookie.Secure < 0 || p.Cookie.Secure > 2 || len(p.Cookie.ExcludeCookies) > 8192 || len(p.DefaultEncoding) > 128 || strings.ContainsAny(p.DefaultEncoding, "\r\n\x00") {
		return errors.New("invalid response policy")
	}
	switch p.Cookie.SameSite {
	case "", "Lax", "Strict", "None":
	default:
		return errors.New("invalid cookie SameSite value")
	}
	c := p.Compression
	if c.MinLength < 0 || c.MinLength > 1<<20 || len(c.IncludeTypes)+len(c.IncludeExtensions)+len(c.ExcludeExtensions)+len(c.ExcludePaths) > 32768 {
		return errors.New("invalid compression options")
	}
	return nil
}

type localResponseWriter struct {
	w        http.ResponseWriter
	state    *forwardRequest
	f        *Forwarder
	body     bytes.Buffer
	status   int
	sent     bool
	noBuffer bool
}

func (w *localResponseWriter) Header() http.Header         { return w.w.Header() }
func (w *localResponseWriter) Unwrap() http.ResponseWriter { return w.w }
func (w *localResponseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	length, _ := strconv.ParseInt(w.Header().Get("Content-Length"), 10, 64)
	if w.noBuffer || status == 206 || status == 101 || length > 1<<20 || strings.Contains(strings.ToLower(w.Header().Get("Content-Type")), "stream") {
		w.pass()
	}
}
func (w *localResponseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	if !w.sent && w.body.Len()+len(data) > 1<<20 {
		w.pass()
	}
	if w.sent {
		return w.w.Write(data)
	}
	return w.body.Write(data)
}
func (w *localResponseWriter) Flush() {
	if w.status == 0 {
		w.status = 200
	}
	w.pass()
	_ = http.NewResponseController(w.w).Flush()
}
func (w *localResponseWriter) pass() {
	if w.sent {
		return
	}
	w.sent = true
	resp := &http.Response{StatusCode: w.status, Header: w.Header(), Body: http.NoBody, ContentLength: -1}
	w.f.transformResponse(w.state, resp)
	w.w.WriteHeader(w.status)
	if w.body.Len() > 0 {
		_, _ = w.w.Write(w.body.Bytes())
		w.body.Reset()
	}
}
func (w *localResponseWriter) finish() {
	if w.status == 0 {
		w.status = 200
	}
	r := w.state.response
	r.status = w.status
	r.location = w.Header().Get("Location")
	r.contentType = w.Header().Get("Content-Type")
	r.encoding = w.Header().Get("Content-Encoding")
	if w.sent {
		return
	}
	resp := &http.Response{StatusCode: w.status, Header: w.Header(), Body: io.NopCloser(bytes.NewReader(w.body.Bytes())), ContentLength: int64(w.body.Len())}
	w.f.transformResponse(w.state, resp)
	w.w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w.w, resp.Body)
	_ = resp.Body.Close()
	w.sent = true
}

func (f *Forwarder) transformResponse(state *forwardRequest, resp *http.Response) {
	s, c := state.snapshot, state.request
	cfg := s.Config.Policy.Network.Response
	if cfg.Cookie.IsEnable == 1 && f.SecureCookie != nil {
		lines := resp.Header.Values("Set-Cookie")
		resp.Header.Del("Set-Cookie")
		for _, line := range lines {
			resp.Header.Add("Set-Cookie", f.SecureCookie(line, RequestScheme(c.Request) == "https", cfg.Cookie))
		}
	}
	p := s.Compiled.Sensitive
	needsSensitive := p != nil && p.mode != ModeOff && (p.out.deny != nil || p.out.replace != nil) && !c.Trusted && !skipAttack(c, "sensitive")
	tamper := matchingTamperRule(s, c)
	privacy := privacyResponseActive(s, c)
	if (!needsSensitive && !privacy && tamper == nil && cfg.Compression.IsEnable != 1) || f.DecodeResponse == nil || f.EncodeResponse == nil || resp.Body == nil || c.Request.Method == "HEAD" || resp.StatusCode == 101 || resp.StatusCode == 206 || resp.Header.Get("Content-Range") != "" || strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "stream") || strings.Contains(strings.ToLower(resp.Header.Get("Cache-Control")), "no-transform") {
		return
	}
	// Unknown-length live streams must not be pre-read while waiting for EOF.
	if resp.ContentLength < 0 || resp.ContentLength > 1<<20 {
		if tamper != nil {
			c.Hit("tamper_skipped", tamper.ID, Low, true, "response exceeds bounded baseline eligibility")
		}
		if needsSensitive {
			c.Hit("sensitive_partial", "sensitive_response", Low, true, "response exceeds bounded transform eligibility")
		}
		return
	}
	body, charset, err := f.DecodeResponse(resp, cfg.DefaultEncoding)
	if err != nil {
		c.Hit("response_transform", "response_transform", Low, true, "bounded response decoding unavailable")
		return
	}
	originalHeader, originalStatus, originalText := resp.Header.Clone(), resp.StatusCode, resp.Status
	transformed := body
	if tamper != nil && resp.StatusCode == 200 {
		transformed = f.tamperResponse(s, c, *tamper, resp, body, &charset)
	}
	changed, deny := sensitiveResponse(s, c, transformed)
	if privacy && !deny {
		changed = privacyResponse(s, c, changed)
	}
	if deny {
		resp.StatusCode = 403
		resp.Status = "403 Forbidden"
		resp.Header.Set("Content-Type", "text/plain; charset=utf-8")
		resp.Header.Del("Content-Encoding")
		resp.Header.Set("Cache-Control", "no-store")
		charset = "utf-8"
		state.response.responseDenied = true
	}
	compression := cfg.Compression
	if bytes.Equal(changed, body) && compression.IsEnable != 1 && !deny {
		return
	}
	encoded := changed
	if !deny {
		encoded, err = f.EncodeResponse(c.Request, resp, changed, charset, compression)
	}
	if err != nil {
		resp.Header, resp.StatusCode, resp.Status = originalHeader, originalStatus, originalText
		c.Hit("response_transform", "response_transform", Low, true, "response encoding failed")
		return
	}
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(encoded))
	resp.ContentLength = int64(len(encoded))
	resp.Header.Set("Content-Length", strconv.Itoa(len(encoded)))
	resp.TransferEncoding = nil
	for _, name := range []string{"ETag", "Content-MD5", "Digest", "Accept-Ranges"} {
		resp.Header.Del(name)
	}
}
