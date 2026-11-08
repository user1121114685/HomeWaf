// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package request

import "HomeWaf/model/common/request"

type WafPrivateGroupAddReq struct {
	PrivateGroupName        string `json:"private_group_name" form:"private_group_name"`
	PrivateGroupBelongCloud string `json:"private_group_belong_cloud" form:"private_group_belong_cloud"`
}
type WafPrivateGroupEditReq struct {
	Id                      string `json:"id"`
	PrivateGroupName        string `json:"private_group_name" form:"private_group_name"`
	PrivateGroupBelongCloud string `json:"private_group_belong_cloud" form:"private_group_belong_cloud"`
}
type WafPrivateGroupDetailReq struct {
	Id string `json:"id"   form:"id"`
}
type WafPrivateGroupDelReq struct {
	Id string `json:"id"   form:"id"`
}
type WafPrivateGroupSearchReq struct {
	request.PageInfo
}
type WafPrivateGroupSearchByCloudReq struct {
	PrivateGroupBelongCloud string `json:"private_group_belong_cloud" form:"private_group_belong_cloud"`
	request.PageInfo
}
