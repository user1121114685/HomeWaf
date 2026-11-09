// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package main

import (
	"HomeWaf/global"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
)

// Software releases default to the HomeWaf repository. Other publisher services are opt-in.
func applyHomeWafEndpoints() {
	for _, endpoint := range []struct {
		name string
		dest *string
	}{
		{"HOMEWAF_UPDATE_URL", &global.GUPDATE_VERSION_URL},
		{"HOMEWAF_RELEASE_URL", &global.GUPDATE_RELEASE_URL},
		{"HOMEWAF_BETA_UPDATE_URL", &global.GUPDATE_GITHUB_VERSION_URL},
		{"HOMEWAF_BOT_DATA_URL", &global.GWAF_BOT_IP_URL_MAIN},
	} {
		value := *endpoint.dest
		if configured, ok := os.LookupEnv(endpoint.name); ok {
			value = configured
		}
		value = strings.TrimSpace(value)
		if value == "" {
			*endpoint.dest = ""
			continue
		}
		u, err := url.Parse(value)
		if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			panic(fmt.Sprintf("invalid %s: expected a base HTTPS URL", endpoint.name))
		}
		ip := net.ParseIP(u.Hostname())
		development := os.Getenv("HOMEWAF_DEV") == "1" && u.Scheme == "http" && ip != nil && ip.IsLoopback()
		if u.Scheme != "https" && !development {
			panic(fmt.Sprintf("invalid %s: HTTPS is required", endpoint.name))
		}
		*endpoint.dest = strings.TrimRight(u.String(), "/")
		if endpoint.name != "HOMEWAF_BETA_UPDATE_URL" && endpoint.name != "HOMEWAF_RELEASE_URL" {
			*endpoint.dest += "/"
		}
	}
}
