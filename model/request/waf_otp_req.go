// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package request

type WafOtpAddReq struct {
	UserName string `json:"user_name" form:"user_name"`
	Url      string `json:"url" form:"url"`
	Secret   string `json:"secret" form:"secret"`
	Issuer   string `json:"issuer" form:"issuer"`
	Remarks  string `json:"remarks" form:"remarks"`
}
type WafOtpEditReq struct {
	Id       string `json:"id" binding:"required,max=64"`
	UserName string `json:"user_name" form:"user_name"`
	Url      string `json:"url" form:"url"`
	Secret   string `json:"secret" form:"secret"`
	Issuer   string `json:"issuer" form:"issuer"`
	Remarks  string `json:"remarks" form:"remarks"`
}
type WafOtpBindReq struct {
	UserName   string `json:"user_name" form:"user_name"`
	Url        string `json:"url" form:"url"`
	Secret     string `json:"secret" form:"secret"`
	Issuer     string `json:"issuer" form:"issuer"`
	Remarks    string `json:"remarks" form:"remarks"`
	SecretCode string `json:"secret_code" form:"secret_code"`
}
type WafOtpDetailReq struct {
	Id string `json:"id"   form:"id" binding:"required,max=64"`
}
type WafOtpDelReq struct {
	Id string `json:"id"   form:"id" binding:"required,max=64"`
}
type WafOtpUnBindReq struct {
	SecretCode string `json:"secret_code" form:"secret_code"`
}
