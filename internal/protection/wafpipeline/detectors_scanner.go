// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package wafpipeline

import (
	"encoding/json"
	"net/http"
	"strings"

	"HomeWaf/internal/protection/wafenginecore/accessgate"
)

var scannerUserAgents = []string{
	"sqlmap", "nikto", "nessus", "acunetix", "appscan",
	"masscan", "wpscan", "gobuster", "dirbuster", "feroxbuster",
	"ffuf", "wfuzz", "zgrab", "openvas", "w3af",
	"skipfish", "whatweb", "netsparker",
	"nuclei", "nmap", "arachni",
}

var scannerHeaderMarkers = []string{
	"x-scan-memo", "x-scanner", "x-wipp", "x-ratproxy-loop", "wvstest", "x-wf-scanner",
}

type scannerDetector struct{ probes []string }

func ScannerDetector(probes []string) Detector { return scannerDetector{probes: probes} }
func (scannerDetector) Name() string           { return "scanner" }
func (scannerDetector) Category() string       { return "scanner" }
func (scannerDetector) NeedsBody() bool        { return false }
func (d scannerDetector) Inspect(v *NormalizedView) []Match {
	if hasScannerSignature(v.Request.UserAgent(), v.Request.Header) {
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

func hasScannerSignature(userAgent string, header http.Header) bool {
	ua := strings.ToLower(userAgent)
	for _, signature := range scannerUserAgents {
		if strings.Contains(ua, signature) {
			return true
		}
	}
	for name := range header {
		for _, marker := range scannerHeaderMarkers {
			if strings.EqualFold(name, marker) {
				return true
			}
		}
	}
	return false
}
