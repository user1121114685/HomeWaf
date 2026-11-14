// Package acmelog routes serialized ACME operations into bounded, redacted logs.
package acmelog

import (
	"net/url"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"

	"HomeWaf/internal/dnsproviders"
)

var scopeMu sync.Mutex

type scope struct{ write func(string, string) }

var current atomic.Pointer[scope]
var urls = regexp.MustCompile(`https?://[^\s<>"']+`)
var pem = regexp.MustCompile(`(?s)-----BEGIN [^-]*PRIVATE KEY-----.*?(-----END [^-]*PRIVATE KEY-----|$)`)
var credentials = regexp.MustCompile(`(?i)(authorization["']?\s*[:=]\s*["']?(?:bearer|basic)\s+|(?:access[_-]?key|api[_-]?key|ddns[_-]?key|token|password|secret|hmac[_-]?key)["']?\s*[:=]\s*["']?)[^\s,;"'\]\}]+`)

func SanitizeValues(message string, secrets ...string) string {
	for _, secret := range secrets {
		if len(secret) >= 4 {
			message = strings.ReplaceAll(message, secret, "[REDACTED]")
			message = strings.ReplaceAll(message, url.QueryEscape(secret), "[REDACTED]")
		}
	}
	return Sanitize(message)
}

func Sanitize(message string) string {
	message = dnsproviders.Redact(message)
	message = pem.ReplaceAllString(message, "[PRIVATE KEY REDACTED]")
	message = urls.ReplaceAllStringFunc(message, func(raw string) string {
		u, err := url.Parse(raw)
		if err != nil {
			return "[URL REDACTED]"
		}
		u.User, u.RawQuery, u.Fragment, u.ForceQuery = nil, "", "", false
		return u.String()
	})
	message = credentials.ReplaceAllString(message, "${1}[REDACTED]")
	message = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, message)
	runes := []rune(message)
	if len(runes) > 2000 {
		message = string(runes[:2000]) + "…"
	}
	return message
}

// Lego's logger has no context parameter. Serialize the scope so concurrent
// callbacks cannot attribute another order's credentials or events to this one.
func Capture(write func(string, string), run func()) {
	scopeMu.Lock()
	defer scopeMu.Unlock()
	current.Store(&scope{write: write})
	defer current.Store(nil)
	run()
}

func Emit(level, message string) {
	if s := current.Load(); s != nil && s.write != nil {
		s.write(level, Sanitize(message))
	}
}
