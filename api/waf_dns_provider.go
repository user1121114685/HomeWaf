package api

import (
	"HomeWaf/global"
	"HomeWaf/internal/dnsproviders"
	"HomeWaf/model"
	"HomeWaf/model/common/response"
	"github.com/gin-gonic/gin"
)

func (w *WafSslOrderApi) DNSProviders(c *gin.Context) {
	response.OkWithDetailed(dnsproviders.List(), "获取成功", c)
}

// DNSCredentialStatus returns only the selected group's keys, never values.
func (w *WafSslOrderApi) DNSCredentialStatus(c *gin.Context) {
	provider, group := c.Query("provider"), c.Query("group")
	if err := dnsproviders.ValidateProvider(provider); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if group == "" {
		response.FailWithMessage("请选择密钥分组", c)
		return
	}
	var entries []struct {
		ID                      string `json:"id"`
		PrivateKey              string `json:"private_key"`
		PrivateGroupName        string `json:"private_group_name"`
		PrivateGroupBelongCloud string `json:"private_group_belong_cloud"`
	}
	err := global.GWAF_LOCAL_DB.Model(&model.PrivateInfo{}).
		Select("id", "private_key", "private_group_name", "private_group_belong_cloud").
		Where("private_group_name = ? AND private_group_belong_cloud = ?", group, provider).Find(&entries).Error
	if err != nil {
		response.FailWithMessage("读取 DNS 配置状态失败", c)
		return
	}
	response.OkWithDetailed(entries, "获取成功", c)
}
