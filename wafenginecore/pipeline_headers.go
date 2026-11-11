package wafenginecore

import (
	"HomeWaf/model"
	"net/http"
)

// The native port proxy shares the existing variable and response-rule
// semantics using copied settings, never a live host/global database lookup.
func PipelineRequestHeaders(headers []model.CustomHeaderItem, r *http.Request, ip string) map[string]string {
	engine := &WafEngine{}
	out := make(map[string]string, len(headers))
	for _, header := range headers {
		out[header.HeaderName] = engine.parseHeaderValue(header.HeaderValue, r, ip)
	}
	return out
}
func PipelineResponseHeaders(rules []model.CustomResponseHeaderRule, r *http.Request, ip string) map[string]string {
	return (&WafEngine{}).applyCustomResponseHeaders(rules, r, ip)
}
