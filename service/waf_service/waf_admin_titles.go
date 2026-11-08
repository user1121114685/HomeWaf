// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package waf_service

import "HomeWaf/wafconfig"

type WafAdminTitlesService struct{}

func (WafAdminTitlesService) Get() wafconfig.AdminTitles {
	return wafconfig.GetAdminTitles()
}

func (WafAdminTitlesService) Update(titles wafconfig.AdminTitles) error {
	return wafconfig.UpdateAdminTitles(titles)
}
