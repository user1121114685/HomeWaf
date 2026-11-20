package request

type WafAccountLogDetailReq struct {
	Id string `json:"id"  form:"id" binding:"required,max=64"` //唯一键
}
