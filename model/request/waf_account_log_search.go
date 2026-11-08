// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package request

import "HomeWaf/model/common/request"

type WafAccountLogSearchReq struct {
	LoginAccount string `json:"login_account" form:"login_account"` //登录账号
	request.PageInfo
}
