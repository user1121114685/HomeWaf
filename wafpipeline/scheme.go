package wafpipeline

import (
	"context"
	"net/http"
)

type externalSchemeKey struct{}

// WithTrustedExternalScheme is used only after checking the TCP peer against
// configured trusted proxies, or when constructing a management dry-run.
func WithTrustedExternalScheme(r *http.Request, scheme string) *http.Request {
	if scheme != "http" && scheme != "https" {
		scheme = "http"
	}
	return r.WithContext(context.WithValue(r.Context(), externalSchemeKey{}, scheme))
}
func RequestScheme(r *http.Request) string {
	if scheme, ok := r.Context().Value(externalSchemeKey{}).(string); ok {
		return scheme
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}
func SameOrigin(source, scheme, host string, referer bool) bool {
	actual, ok := canonicalOrigin(source, referer)
	expected, valid := canonicalOrigin(scheme+"://"+host, false)
	return ok && valid && actual == expected
}
