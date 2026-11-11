package wafpipeline

import (
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

func compileBusinessSelector(config PortConfig) *ProxyTarget {
	root := &ProxyTarget{}
	for i := range config.Routes {
		route := &config.Routes[i]
		hosts := make([]string, len(route.Hosts))
		for i, h := range route.Hosts {
			hosts[i] = strings.ToLower(strings.TrimSuffix(h, "."))
		}
		entry := preparedBusinessRoute{hosts: hosts, prefix: route.PathPrefix, match: route.MatchType, priority: route.Priority, target: &ProxyTarget{configRoute: route}}
		if route.MatchType == "regex" {
			entry.expression, _ = regexp.Compile(route.PathPrefix)
		}
		root.routes = append(root.routes, entry)
	}
	return root
}
func httpsRedirectLocation(r *http.Request, port uint16) string {
	u, err := url.Parse("http://" + r.Host)
	if err != nil || u.Hostname() == "" || u.User != nil {
		return ""
	}
	host := u.Hostname()
	if port != 443 {
		host = net.JoinHostPort(host, strconv.Itoa(int(port)))
	} else if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return (&url.URL{Scheme: "https", Host: host, Path: r.URL.Path, RawPath: r.URL.RawPath, RawQuery: r.URL.RawQuery}).String()
}
func businessEntryStep(s *Snapshot) Step {
	return Step{Number: 2, Name: "local_business_entry", FailClosed: true, Inspect: func(c *RequestContext) (Verdict, error) {
		route := s.Compiled.BusinessSelector.forRequest(c.Request).configRoute
		if route == nil {
			return Verdict{Action: Continue}, nil
		}
		if route.TargetType == "unavailable" {
			return Verdict{Action: Deny, Status: 503, Reason: "business_disabled"}, nil
		}
		if route.TargetType == "https_redirect" {
			code := route.RedirectCode
			if code == 0 {
				code = 301
			}
			location := httpsRedirectLocation(c.Request, route.RedirectPort)
			if location == "" {
				return Verdict{Action: Deny, Status: 400, Reason: "invalid_authority"}, nil
			}
			c.Values["https_redirect"] = location
			return Verdict{Action: Allow, Status: code, Reason: "https_redirect"}, nil
		}
		return Verdict{Action: Continue}, nil
	}}
}
func requestProxyHop(r *http.Request) int {
	values := r.Header.Values("waf_req_hop")
	if len(values) != 1 || len(values[0]) > 10 {
		return 0
	}
	n, err := strconv.Atoi(values[0])
	if err != nil || n < 0 {
		return 0
	}
	return n
}
func proxyLoopStep(s *Snapshot) Step {
	return Step{Number: 5, Name: "proxy_loop", Inspect: func(c *RequestContext) (Verdict, error) {
		if limit := s.Config.Policy.Network.MaxProxyHops; limit > 0 && requestProxyHop(c.Request) >= limit {
			c.Hit("proxy_loop", "proxy_loop", Low, true, "configured proxy hop limit reached")
			return Verdict{Action: Deny, Status: 508, Reason: "proxy_loop"}, nil
		}
		return Verdict{Action: Continue}, nil
	}}
}
