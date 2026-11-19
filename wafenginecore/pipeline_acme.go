package wafenginecore

import (
	"HomeWaf/wafacme"
	"HomeWaf/wafpipeline"
	"net/url"
	"slices"
	"strings"
)

func PipelineCertificateChallenge(s *wafpipeline.Snapshot, c *wafpipeline.RequestContext) (string, bool) {
	r := c.Request
	token := acmeChallengeToken(r)
	source := s.Config.HostCode
	if token == "" || source == "" {
		return "", false
	}
	host := strings.TrimSuffix((&url.URL{Host: r.Host}).Hostname(), ".")
	if host == "" || strings.Contains(host, "*") {
		return "", false
	}
	matches := func(name string) bool {
		return strings.EqualFold(strings.TrimSuffix(name, "."), host)
	}
	allowed := slices.ContainsFunc(s.Config.HostNames, matches)
	if !allowed {
		for _, route := range s.Config.Routes {
			if slices.ContainsFunc(route.Hosts, matches) {
				allowed = true
				break
			}
		}
	}
	if !allowed {
		return "", false
	}
	if c.DryRun {
		return wafacme.Lookup(source, token)
	}
	return loadChallengeContent(source, token)
}
