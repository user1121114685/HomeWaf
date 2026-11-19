package wafenginecore

import (
	"HomeWaf/model"
	"net/http"
)

func PipelineEncodeResponse(req *http.Request, resp *http.Response, body []byte, charset string, cfg model.ResponseCompressConfig) ([]byte, error) {
	encoded, err := compressContent(resp, body, charset)
	if err != nil {
		return nil, err
	}
	return maybeApplyResponseCompress(req, resp, encoded, cfg), nil
}
func PipelineSecureCookie(raw string, secure bool, cfg model.CookieSecurityConfig) string {
	return secureSetCookie(raw, secure, cfg)
}
