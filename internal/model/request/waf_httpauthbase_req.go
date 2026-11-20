// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package request

import "HomeWaf/internal/model/common/request"

type WafHttpAuthBaseAddReq struct {
	HostCode string `json:"host_code" form:"host_code"`
	UserName string `json:"user_name" form:"user_name"`
	Password string `json:"password" form:"password"`
}
type WafHttpAuthBaseEditReq struct {
	Id       string `json:"id" binding:"required,max=64"`
	HostCode string `json:"host_code" form:"host_code"`
	UserName string `json:"user_name" form:"user_name"`
	Password string `json:"password" form:"password"`
}
type WafHttpAuthBaseDetailReq struct {
	Id string `json:"id"   form:"id" binding:"required,max=64"`
}
type WafHttpAuthBaseDelReq struct {
	Id string `json:"id"   form:"id" binding:"required,max=64"`
}
type WafHttpAuthBaseSearchReq struct {
	HostCode string `json:"host_code" form:"host_code"`
	UserName string `json:"user_name" form:"user_name"`
	request.PageInfo
}
