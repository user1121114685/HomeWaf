package wafenginecore

import (
	"HomeWaf/utils"
	"HomeWaf/wafacme"
	"HomeWaf/wafpipeline"
	"net/url"
	"strings"
)

func PipelineCertificateChallenge(s *wafpipeline.Snapshot, c *wafpipeline.RequestContext) (string, bool) {
	r := c.Request
	const prefix = "/.well-known/acme-challenge/"
	if r.Method != "GET" && r.Method != "HEAD" || !strings.HasPrefix(r.URL.Path, prefix) || r.URL.EscapedPath() != r.URL.Path || r.URL.RawQuery != "" {
		return "", false
	}
	token := strings.TrimPrefix(r.URL.Path, prefix)
	if !utils.IsValidChallengeFile(token) || strings.ContainsAny(token, "./\\") {
		return "", false
	}
	u, err := url.Parse("http://" + r.Host)
	if err != nil {
		return "", false
	}
	source := s.Config.ACMEHosts[strings.ToLower(u.Hostname())]
	if source == "" {
		return "", false
	}
	if c.DryRun {
		return wafacme.Lookup(source, token)
	}
	return loadChallengeContent(source, token)
}
