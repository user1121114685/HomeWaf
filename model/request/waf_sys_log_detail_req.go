package request

type WafSysLogDetailReq struct {
	Id string `json:"id"  form:"id" binding:"required,max=64"` //唯一键
}
