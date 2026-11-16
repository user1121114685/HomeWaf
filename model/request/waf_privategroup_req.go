// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package request

import "HomeWaf/model/common/request"

type WafPrivateGroupAddReq struct {
	PrivateGroupName        string `json:"private_group_name" form:"private_group_name"`
	PrivateGroupBelongCloud string `json:"private_group_belong_cloud" form:"private_group_belong_cloud"`
}
type WafPrivateGroupEditReq struct {
	Id                      string `json:"id" binding:"required,max=64"`
	PrivateGroupName        string `json:"private_group_name" form:"private_group_name"`
	PrivateGroupBelongCloud string `json:"private_group_belong_cloud" form:"private_group_belong_cloud"`
}
type WafPrivateGroupDetailReq struct {
	Id string `json:"id"   form:"id" binding:"required,max=64"`
}
type WafPrivateGroupDelReq struct {
	Id string `json:"id"   form:"id" binding:"required,max=64"`
}
type WafPrivateGroupSearchReq struct {
	request.PageInfo
}
type WafPrivateGroupSearchByCloudReq struct {
	PrivateGroupBelongCloud string `json:"private_group_belong_cloud" form:"private_group_belong_cloud"`
	request.PageInfo
}
