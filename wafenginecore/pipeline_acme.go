package wafenginecore

import (
	"HomeWaf/wafacme"
	"HomeWaf/wafpipeline"
	"net/url"
	"strings"
)

func PipelineCertificateChallenge(s *wafpipeline.Snapshot, c *wafpipeline.RequestContext) (string, bool) {
	r := c.Request
	token := acmeChallengeToken(r)
	if token == "" {
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
