package mcp

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
)

var credentialText = regexp.MustCompile(`(?im)^(\s*(?:authorization|proxy-authorization|cookie|set-cookie|x-api-key)\s*:\s*)[^\r\n]+`)
var credentialValue = regexp.MustCompile(`(?i)((?:password|passwd|secret|access_token|refresh_token|api_key|apikey|session_id|token)["']?\s*[:=]\s*["']?)[^\s&"'<>;,}]+`)
var privateMaterial = regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`)
var bearer = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]+`)

func secretField(key string) bool {
	k := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", "_"), " ", "_"))
	for _, part := range []string{"password", "passwd", "secret", "private_key", "privatekey", "credential", "authorization", "proxy_authorization", "cookie", "cookies", "set_cookie", "api_key", "apikey", "token_hash", "gate_key", "tls_key", "signing_key", "service_token_hashes"} {
		if k == part {
			return true
		}
	}
	for _, suffix := range []string{"_password", "_secret", "_private_key", "_api_key", "_token_hash"} {
		if strings.HasSuffix(k, suffix) {
			return true
		}
	}
	return k == "token" || k == "access_token" || k == "refresh_token"
}
func scrubText(s string) string {
	s = privateMaterial.ReplaceAllString(s, "[REDACTED PRIVATE KEY]")
	s = credentialText.ReplaceAllString(s, "${1}[REDACTED]")
	s = credentialValue.ReplaceAllString(s, "${1}[REDACTED]")
	s = bearer.ReplaceAllString(s, "Bearer [REDACTED]")
	if u, err := url.Parse(s); err == nil && u.Host != "" {
		if u.User != nil {
			u.User = url.User("REDACTED")
		}
		q := u.Query()
		changed := false
		for k := range q {
			if secretField(k) {
				q.Set(k, "[REDACTED]")
				changed = true
			}
		}
		if changed {
			u.RawQuery = q.Encode()
		}
		s = u.String()
	}
	return s
}
func redact(value any) any {
	switch v := value.(type) {
	case map[string]any:
		for k, x := range v {
			if k == "src_byte_body" || k == "src_byte_res_body" || k == "src_url" {
				// Raw byte fields duplicate the decoded payload and can otherwise
				// bypass field redaction via base64 encoding.
				v[k] = "[Use the decoded, redacted body/res_body/url fields]"
			} else if secretField(k) && x != nil {
				// A cookie policy is configuration, not a cookie value. Preserve
				// structured settings so a read/edit/save round trip is lossless.
				switch x.(type) {
				case string, []any:
					v[k] = "[REDACTED]"
				default:
					v[k] = redact(x)
				}
			} else {
				v[k] = redact(x)
			}
		}
	case []any:
		for i, x := range v {
			v[i] = redact(x)
		}
	case string:
		var nested any
		trim := strings.TrimSpace(v)
		if (strings.HasPrefix(trim, "{") || strings.HasPrefix(trim, "[")) && json.Unmarshal([]byte(v), &nested) == nil {
			b, _ := json.Marshal(redact(nested))
			return string(b)
		}
		return scrubText(v)
	}
	return value
}
