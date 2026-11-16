// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package request

import "HomeWaf/model/common/request"

type WafSslorderaddReq struct {
	AutoRenew        *bool  `json:"auto_renew,omitempty"`
	RequestID        string `json:"request_id" binding:"omitempty,max=128"`
	HostCode         string `json:"host_code" binding:"required,max=64"`            //网站唯一码（主要键）
	ApplyPlatform    string `json:"apply_platform" binding:"omitempty,max=100"`     //申请平台，取CA服务器名称（ca_server_name），默认 letsencrypt
	ApplyMethod      string `json:"apply_method" enums:"http01,dns01"`              //申请方式：http01(HTTP 文件验证) 或 dns01(DNS 验证)
	SkipDNSVerify    int64  `json:"skip_dns_verify"`                                //dns01是否跳过本地DNS传播校验 0否 1是
	ApplyDns         string `json:"apply_dns" binding:"omitempty,max=100"`          //申请dns服务商
	ApplyEmail       string `json:"apply_email" binding:"required,email,max=255"`   //申请邮箱
	ApplyDomain      string `json:"apply_domain" binding:"required,max=8192"`       //申请域名
	PrivateGroupName string `json:"private_group_name" binding:"omitempty,max=255"` //密钥分组名称DNS有效
}
type WafSslordereditReq struct {
	AutoRenew        *bool  `json:"auto_renew,omitempty"`
	RequestID        string `json:"request_id" binding:"omitempty,max=128"`
	Id               string `json:"id" binding:"required,max=64"`
	HostCode         string `json:"host_code" binding:"required,max=64"`            //网站唯一码（主要键）
	ApplyPlatform    string `json:"apply_platform" binding:"omitempty,max=100"`     //申请平台，取CA服务器名称（ca_server_name），默认 letsencrypt
	ApplyMethod      string `json:"apply_method" enums:"http01,dns01"`              //申请方式：http01(HTTP 文件验证) 或 dns01(DNS 验证)
	SkipDNSVerify    int64  `json:"skip_dns_verify"`                                //dns01是否跳过本地DNS传播校验 0否 1是
	ApplyDns         string `json:"apply_dns" binding:"omitempty,max=100"`          //申请dns服务商
	ApplyEmail       string `json:"apply_email" binding:"required,email,max=255"`   //申请邮箱
	ApplyDomain      string `json:"apply_domain" binding:"required,max=8192"`       //申请域名
	ApplyStatus      string `json:"apply_status"`                                   //申请状态
	PrivateGroupName string `json:"private_group_name" binding:"omitempty,max=255"` //密钥分组名称DNS有效
}
type WafSslorderdetailReq struct {
	Id string `json:"id"   form:"id" binding:"required,max=64"`
}
type WafSslorderdeleteReq struct {
	Id string `json:"id"   form:"id" binding:"required,max=64"`
}
type WafSslorderretryReq struct {
	Id        string `json:"id" binding:"required,max=64"`
	RequestID string `json:"request_id" binding:"required,max=128"`
}
type WafSslordersearchReq struct {
	CertificateID string `json:"certificate_id" binding:"omitempty,max=64"`
	Trigger       string `json:"trigger" binding:"omitempty,oneof=initial automatic manual"`
	ApplyStatus   string `json:"apply_status" binding:"omitempty,oneof=submitted applying success fail renewed expired"`
	ApplyDomain   string `json:"apply_domain" binding:"omitempty,max=8192"` //可选：按申请域名筛选
	HostCode      string `json:"host_code" binding:"omitempty,max=64"`      //可选：按关联服务筛选
	request.PageInfo
}
