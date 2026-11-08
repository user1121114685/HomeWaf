// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package request

import "HomeWaf/model/common/request"

type WafLoadBalanceSearchReq struct {
	HostCode    string `json:"host_code" `  //主机码
	Remote_port int    `json:"remote_port"` //远端端口
	Remote_ip   string `json:"remote_ip"`   //远端指定IP
	request.PageInfo
}
