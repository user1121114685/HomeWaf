// HomeWaf modification (2026-11-14): service identities for log and analysis selectors.
package waf_service

import (
	"HomeWaf/internal/model/response"
	"fmt"
	"strconv"
	"strings"
)

// GetLogServiceOptions exposes labels and log identities, never policies or secrets.
func (s *WafHostService) GetLogServiceOptions() ([]response.AllHostRep, error) {
	configs, err := WafPipelineServiceApp.List()
	if err != nil {
		return nil, err
	}
	options := make([]response.AllHostRep, 0, len(configs))
	for _, c := range configs {
		code := c.HostCode
		if code == "" {
			code = "pipeline-" + strconv.Itoa(int(c.Port))
		}
		name := strings.TrimSpace(c.Name)
		if name == "" {
			name = fmt.Sprintf("服务 :%d", c.Port)
		}
		options = append(options, response.AllHostRep{Code: code, Host: fmt.Sprintf("%s · :%d", name, c.Port), Nickname: name, Source: "pipeline", Port: int(c.Port)})
	}
	return options, nil
}
