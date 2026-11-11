// Package logsanitize defines the mandatory credential boundary for log sinks.
// It never changes the HTTP request or response delivered to the application.
package logsanitize

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

const Marker = "[MASKED]"
const MaxBytes = 64 << 10

func Cut(s string) string {
	if len(s) <= MaxBytes {
		return s
	}
	return strings.ToValidUTF8(s[:MaxBytes], "")
}

func Sensitive(name string) bool {
	for range 3 {
		decoded, err := url.QueryUnescape(name)
		if err != nil || decoded == name {
			break
		}
		name = decoded
	}
	name = strings.ToLower(name)
	for _, key := range []string{"password", "passwd", "pwd", "passphrase", "secret", "token", "credential", "authorization", "api-key", "api_key", "apikey", "session", "jwt", "signature", "private_key", "access_key"} {
		if strings.Contains(name, key) {
			return true
		}
	}
	return name == "key" || name == "sid" || name == "auth" || name == "code" || name == "otp" || name == "totp" || name == "otp_code" || name == "totp_code"
}

func Query(raw string) string {
	if len(raw) > MaxBytes {
		return Marker
	}
	parts := strings.Split(raw, "&")
	for i, p := range parts {
		k, _, ok := strings.Cut(p, "=")
		if ok && Sensitive(k) {
			parts[i] = k + "=" + Marker
		}
	}
	return strings.Join(parts, "&")
}

func URI(raw string) string {
	if len(raw) > MaxBytes {
		return Marker
	}
	if u, err := url.Parse(raw); err == nil && u.User != nil {
		u.User = nil
		raw = u.String()
	}
	path, query, ok := strings.Cut(raw, "?")
	if !ok {
		return raw
	}
	return path + "?" + Query(query)
}

func Headers(raw string) string {
	if len(raw) > MaxBytes {
		return Marker
	}
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			lines[i] = Marker
			continue
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		n := strings.ToLower(strings.TrimSpace(name))
		if n == "cookie" || n == "set-cookie" || Sensitive(n) || strings.Contains(n, "auth") && n != "www-authenticate" {
			lines[i] = name + ": " + Marker
		} else if n == "referer" || n == "location" {
			lines[i] = name + ": " + URI(strings.TrimSpace(value))
		}
	}
	return strings.Join(lines, "\r\n")
}

func Cookies(raw string) string {
	if raw == "" {
		return ""
	}
	if len(raw) > MaxBytes {
		return Marker
	}
	var cookies []http.Cookie
	if json.Unmarshal([]byte(raw), &cookies) != nil {
		return Marker
	}
	for i := range cookies {
		cookies[i].Value = Marker
		cookies[i].Raw = ""
		cookies[i].RawExpires = ""
		cookies[i].Unparsed = nil
	}
	b, err := json.Marshal(cookies)
	if err != nil {
		return Marker
	}
	return string(b)
}

func redactJSON(v any, depth int) any {
	if depth > 32 {
		return Marker
	}
	switch value := v.(type) {
	case map[string]any:
		for key, child := range value {
			if Sensitive(key) {
				value[key] = Marker
			} else {
				value[key] = redactJSON(child, depth+1)
			}
		}
	case []any:
		for i, child := range value {
			value[i] = redactJSON(child, depth+1)
		}
	}
	return v
}

// Body retains structured, bounded evidence only. Unknown, multipart, binary,
// truncated and invalid JSON cannot be safely classified and are omitted.
func Body(raw string) string {
	if raw == "" || raw == Marker {
		return raw
	}
	if len(raw) > MaxBytes {
		return Marker
	}
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		var v any
		if json.Unmarshal([]byte(raw), &v) != nil {
			return Marker
		}
		out, err := json.Marshal(redactJSON(v, 0))
		if err != nil {
			return Marker
		}
		return string(out)
	}
	if strings.Contains(raw, "=") && !strings.ContainsAny(raw, "\r\n<>") {
		if _, err := url.ParseQuery(raw); err == nil {
			return Query(raw)
		}
	}
	return Marker
}
