// HomeWaf modification (2026-11-14): service identities for log and analysis selectors.
package waf_service

import (
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/model/response"
	"fmt"
	"strconv"
	"strings"
)

// GetLogServiceOptions exposes labels and log identities, never policies or secrets.
func (s *WafHostService) GetLogServiceOptions() ([]response.AllHostRep, error) {
	var hosts []model.Hosts
	if err := global.GWAF_LOCAL_DB.Order("global_host asc").Find(&hosts).Error; err != nil {
		return nil, err
	}
	configs, err := WafPipelineServiceApp.List()
	if err != nil {
		return nil, err
	}
	options := make([]response.AllHostRep, 0, len(hosts)+len(configs))
	seen := make(map[string]bool)
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
		seen[code] = true
	}
	for _, host := range hosts {
		if seen[host.Code] {
			continue
		}
		options = append(options, response.AllHostRep{Code: host.Code, Host: host.DisplayName(), Nickname: host.Nickname, PreHost: fmt.Sprintf("%s:%d", host.Host, host.Port), GlobalHost: host.GLOBAL_HOST, GroupCode: host.GroupCode, Source: "host", Port: host.Port})
	}
	return options, nil
}
