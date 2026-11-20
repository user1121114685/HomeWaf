package wafpipeline

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

type TamperRule struct {
	ID          string   `json:"id"`
	Path        string   `json:"path"`
	Hosts       []string `json:"hosts,omitempty"`
	Mode        Mode     `json:"mode"`
	Action      string   `json:"action"`
	IgnoreQuery bool     `json:"ignore_query"`
	MaxBytes    int      `json:"max_bytes"`
	Generation  uint64   `json:"generation"`
	LearnedAt   int64    `json:"learned_at,omitempty"`
}
type TamperBaseline struct {
	Binding     string `json:"binding"`
	Content     []byte `json:"content"`
	ContentType string `json:"content_type"`
	Charset     string `json:"charset"`
	Hash        string `json:"hash"`
	LearnedAt   int64  `json:"learned_at"`
}

func TamperBinding(rule TamperRule) string {
	raw, _ := json.Marshal(struct {
		ID, Path   string
		Hosts      []string
		Generation uint64
	}{rule.ID, rule.Path, rule.Hosts, rule.Generation})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func NewTamperBaseline(rule TamperRule, body []byte, contentType, charset string) TamperBaseline {
	sum := sha256.Sum256(body)
	return TamperBaseline{Binding: TamperBinding(rule), Content: append([]byte{}, body...), ContentType: contentType, Charset: charset, Hash: hex.EncodeToString(sum[:]), LearnedAt: time.Now().Unix()}
}
func validateTamperRules(rules []TamperRule) error {
	if len(rules) > 128 {
		return errors.New("too many tamper rules")
	}
	ids := map[string]bool{}
	for _, r := range rules {
		if r.ID == "" || len(r.ID) > 128 || ids[r.ID] || !ValidMode(r.Mode) || (r.Action != "replace" && r.Action != "alert") || r.MaxBytes < 0 || r.MaxBytes > 1<<20 || !strings.HasPrefix(r.Path, "/") || strings.ContainsAny(r.Path, "?#*\r\n") || len(r.Path) > 2048 {
			return errors.New("invalid tamper rule")
		}
		if err := ValidateScope(Scope{Hosts: r.Hosts}); err != nil {
			return err
		}
		ids[r.ID] = true
	}
	return nil
}
func ValidateTamperBaselines(baselines map[string]TamperBaseline) error {
	if len(baselines) > 128 {
		return errors.New("too many tamper baselines")
	}
	total := 0
	for id, b := range baselines {
		total += len(b.Content)
		sum := sha256.Sum256(b.Content)
		if len(id) > 128 || len(b.ContentType) > 1024 || len(b.Charset) > 128 || len(b.Binding) != 64 || hex.EncodeToString(sum[:]) != b.Hash || total > 1<<20 {
			return errors.New("invalid tamper baseline or port 1MiB quota exceeded")
		}
	}
	return nil
}
func matchingTamperRule(s *Snapshot, c *RequestContext) *TamperRule {
	if c.Request.Method != "GET" || !TamperPublicRequest(c.Request) {
		return nil
	}
	for _, rule := range s.Config.Policy.Network.Response.Tamper {
		if rule.Mode != ModeOff && rule.Path == c.Request.URL.Path && (rule.IgnoreQuery || c.Request.URL.RawQuery == "") && scopeMatches(Scope{Hosts: rule.Hosts}, c.Request) {
			copy := rule
			return &copy
		}
	}
	return nil
}
func (f *Forwarder) tamperResponse(s *Snapshot, c *RequestContext, rule TamperRule, resp *http.Response, body []byte, charset *string) []byte {
	if !TamperPublicResponse(c.Request, resp) {
		c.Hit("tamper_skipped", rule.ID, Low, true, "personalized response excluded from baseline policy")
		return body
	}
	limit := rule.MaxBytes
	if limit == 0 {
		limit = 1 << 20
	}
	if len(body) > limit {
		c.Hit("tamper_skipped", rule.ID, Low, true, "baseline body quota exceeded")
		return body
	}
	baseline, ok := s.Secrets.TamperBaselines[rule.ID]
	if !ok || baseline.Binding != TamperBinding(rule) {
		if !c.DryRun && f.CaptureTamper != nil && f.CaptureTamper(s, rule, body, resp.Header.Get("Content-Type"), *charset) {
			c.Hit("tamper_learning", rule.ID, Low, true, "baseline queued for bounded persistence")
		} else {
			c.Hit("tamper_skipped", rule.ID, Low, true, "baseline learning unavailable or already pending")
		}
		return body
	}
	sum := sha256.Sum256(body)
	if hex.EncodeToString(sum[:]) == baseline.Hash {
		return body
	}
	// Server content tampering must not ban the visitor who discovers it.
	c.Hit("tamper_detected", rule.ID, High, true, "response differs from the configured baseline")
	if rule.Mode == ModeObserve || rule.Action == "alert" {
		return body
	}
	resp.Header.Set("Content-Type", baseline.ContentType)
	*charset = baseline.Charset
	return baseline.Content
}

func TamperPublicRequest(r *http.Request) bool {
	return r != nil && r.Header.Get("Cookie") == "" && r.Header.Get("Authorization") == ""
}
func TamperPublicResponse(r *http.Request, response *http.Response) bool {
	control := strings.ToLower(response.Header.Get("Cache-Control"))
	return TamperPublicRequest(r) && response.Header.Get("Set-Cookie") == "" && response.Header.Get("Vary") == "" && !strings.Contains(control, "private") && !strings.Contains(control, "no-store")
}
func bodyResponsePolicyActive(s *Snapshot, c *RequestContext) bool {
	return sensitiveResponseActive(s, c) || privacyResponseActive(s, c) || matchingTamperRule(s, c) != nil
}
