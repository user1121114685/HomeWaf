// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package request

import "HomeWaf/internal/model/common/request"

type WafCaServerInfoAddReq struct {
	CaServerName    string `json:"ca_server_name" form:"ca_server_name"`
	CaServerAddress string `json:"ca_server_address" form:"ca_server_address"`
	Remarks         string `json:"remarks" form:"remarks"`
}
type WafCaServerInfoEditReq struct {
	Id string `json:"id" binding:"required,max=64"`

	CaServerName    string `json:"ca_server_name" form:"ca_server_name"`
	CaServerAddress string `json:"ca_server_address" form:"ca_server_address"`
	Remarks         string `json:"remarks" form:"remarks"`
}
type WafCaServerInfoDetailReq struct {
	Id string `json:"id"   form:"id" binding:"required,max=64"`
}
type WafCaServerInfoDelReq struct {
	Id string `json:"id"   form:"id" binding:"required,max=64"`
}
type WafCaServerInfoSearchReq struct {
	request.PageInfo
}
