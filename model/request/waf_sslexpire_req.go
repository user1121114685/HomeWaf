// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package request

import "HomeWaf/model/common/request"

type WafSslExpireAddReq struct {
	Domain   string `json:"domain" form:"domain"`
	Port     int    `json:"port" form:"port"`
	VisitLog string `json:"visit_log" form:"visit_log"`
	Status   string `json:"status" form:"status"`
}
type WafSslExpireEditReq struct {
	Id string `json:"id" binding:"required,max=64"`

	Domain   string `json:"domain" form:"domain"`
	Port     int    `json:"port" form:"port"`
	VisitLog string `json:"visit_log" form:"visit_log"`
	Status   string `json:"status" form:"status"`
}
type WafSslExpireDetailReq struct {
	Id string `json:"id"   form:"id" binding:"required,max=64"`
}
type WafSslExpireDelReq struct {
	Id string `json:"id"   form:"id" binding:"required,max=64"`
}
type WafSslExpireSearchReq struct {
	Domain string `json:"domain" form:"domain"`
	request.PageInfo
}
