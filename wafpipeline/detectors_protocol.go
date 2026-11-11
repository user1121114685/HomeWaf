package wafpipeline

import (
	lib "HomeWaf/libinjection-go"
	"HomeWaf/wafenginecore/accessgate"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/http/httpguts"
)

type protocolDetector struct {
	methods   []string
	maxHeader int
}

func ProtocolDetector(methods []string, maxHeader int) Detector {
	return protocolDetector{methods: methods, maxHeader: maxHeader}
}
func (protocolDetector) Name() string     { return "malformed" }
func (protocolDetector) Category() string { return "malformed" }
func (protocolDetector) NeedsBody() bool  { return false }
func (d protocolDetector) Inspect(v *NormalizedView) []Match {
	r := v.Request
	hit := func(code string) []Match { return []Match{{"malformed", "request_headers", code}} }
	cl := r.Header.Values("Content-Length")
	te := r.Header.Values("Transfer-Encoding")
	if (len(te) > 0 || len(r.TransferEncoding) > 0) && (len(cl) > 0 || r.ContentLength > 0) {
		return hit("conflicting_framing")
	}
	var length string
	for _, line := range cl {
		for _, value := range strings.Split(line, ",") {
			value = strings.TrimSpace(value)
			if n, err := strconv.ParseInt(value, 10, 64); err != nil || n < 0 {
				return hit("invalid_content_length")
			}
			if length != "" && value != length {
				return hit("conflicting_content_length")
			}
			length = value
		}
	}
	for _, encoding := range append(append([]string{}, te...), r.TransferEncoding...) {
		if !strings.EqualFold(strings.TrimSpace(encoding), "chunked") {
			return hit("ambiguous_transfer_encoding")
		}
	}
	if len(r.TransferEncoding) > 1 || len(te) > 1 {
		return hit("duplicate_transfer_encoding")
	}
	if len(r.Header.Values("Host")) > 1 {
		return hit("duplicate_host")
	}
	total := len(r.RequestURI) + len(r.Method) + len(r.Host)
	for k, values := range r.Header {
		if !httpguts.ValidHeaderFieldName(k) {
			return hit("invalid_header_name")
		}
		for _, value := range values {
			total += len(k) + len(value) + 4
			if !httpguts.ValidHeaderFieldValue(value) {
				return hit("header_injection")
			}
		}
	}
	if strings.ContainsAny(r.Host, "\x00\r\n") || strings.ContainsAny(r.RequestURI, "\x00\r\n") {
		return hit("request_line_injection")
	}
	for _, raw := range []string{r.URL.EscapedPath(), r.URL.RawQuery} {
		decoded, err := url.QueryUnescape(raw)
		if err != nil {
			return hit("protocol_observe")
		}
		if strings.ContainsRune(decoded, 0) {
			return hit("nul_in_request")
		}
	}
	if total > d.maxHeader || len(r.Header) > 128 {
		return hit("protocol_observe")
	}
	if !(r.ProtoMajor == 1 && (r.ProtoMinor == 0 || r.ProtoMinor == 1) || r.ProtoMajor == 2 && r.ProtoMinor == 0) {
		return hit("protocol_observe")
	}
	for _, m := range d.methods {
		if r.Method == m {
			return nil
		}
	}
	return hit("protocol_observe")
}

type scannerDetector struct{ probes []string }

func ScannerDetector(probes []string) Detector { return scannerDetector{probes: probes} }
func (scannerDetector) Name() string           { return "scanner" }
func (scannerDetector) Category() string       { return "scanner" }
func (scannerDetector) NeedsBody() bool        { return false }
func (d scannerDetector) Inspect(v *NormalizedView) []Match {
	names := make([]string, 0, len(v.Request.Header))
	for k := range v.Request.Header {
		names = append(names, k)
	}
	if lib.HasScannerSignature(v.Request.UserAgent(), names) {
		return []Match{{"scanner", "request_headers", "scanner_signature"}}
	}
	if accessgate.MatchPathPrefix(v.Path, d.probes) {
		return []Match{{"scanner", "path", "configured_absent_path"}}
	}
	return nil
}
func scannerPaths(rule Rule) []string {
	var paths []string
	if raw, ok := rule.Params["probe_paths"]; ok {
		_ = json.Unmarshal(raw, &paths)
	}
	return paths
}
