package wafenginecore

import (
	"HomeWaf/model"
	"net/http"
)

// Stateless adapters retain the existing bounded codec and cookie policies.
func PipelineDecodeResponse(resp *http.Response, encoding string) ([]byte, string, error) {
	return (&WafEngine{}).getOrgContent(resp, false, encoding)
}
func PipelineEncodeResponse(req *http.Request, resp *http.Response, body []byte, charset string, cfg model.ResponseCompressConfig) ([]byte, error) {
	engine := &WafEngine{}
	encoded, err := engine.compressContent(resp, false, body, charset)
	if err != nil {
		return nil, err
	}
	return engine.maybeApplyResponseCompress(req, resp, encoded, cfg), nil
}
func PipelineSecureCookie(raw string, secure bool, cfg model.CookieSecurityConfig) string {
	return secureSetCookie(raw, secure, cfg)
}
