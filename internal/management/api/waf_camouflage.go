package api

import (
	"HomeWaf/internal/model/common/response"
	"HomeWaf/internal/platform/utils"
	"HomeWaf/internal/protection/camouflage"
	"HomeWaf/internal/services/waf_service"
	"strconv"

	"github.com/gin-gonic/gin"
)

type WafCamouflageApi struct{}

func (w *WafCamouflageApi) Config(c *gin.Context) {
	service := waf_service.WafCamouflageServiceApp
	service.ObserveManagementClientIP(utils.GetManageClientIP(c))
	response.OkWithDetailed(gin.H{"config": service.Config(), "status": serviceStatus(service)}, "", c)
}

func serviceStatus(service *waf_service.WafCamouflageService) gin.H {
	return gin.H{"profiles": service.Status(), "local_cidrs": service.LocalCIDRs()}
}

func (w *WafCamouflageApi) Profiles(c *gin.Context) {
	response.OkWithDetailed(serviceProfiles(), "", c)
}

func serviceProfiles() []camouflage.Profile {
	return waf_service.WafCamouflageServiceApp.Profiles()
}

func (w *WafCamouflageApi) Save(c *gin.Context) {
	var config camouflage.DefenseConfig
	if !pipelineBind(c, &config) {
		return
	}
	service := waf_service.WafCamouflageServiceApp
	service.ObserveManagementClientIP(utils.GetManageClientIP(c))
	operator := pipelineOperator(c)
	result, err := service.Save(config, operator)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithDetailed(gin.H{"config": result, "status": serviceStatus(service)}, "saved", c)
}

func (w *WafCamouflageApi) Status(c *gin.Context) {
	response.OkWithDetailed(serviceStatus(waf_service.WafCamouflageServiceApp), "", c)
}

func (w *WafCamouflageApi) Events(c *gin.Context) {
	limit := 100
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 500 {
			response.FailWithMessage("limit must be between 1 and 500", c)
			return
		}
		limit = parsed
	}
	rows, err := waf_service.WafCamouflageServiceApp.Events(limit)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithDetailed(rows, "", c)
}

func (w *WafCamouflageApi) ClearEvents(c *gin.Context) {
	if err := waf_service.WafCamouflageServiceApp.ClearEvents(); err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithMessage("events cleared", c)
}
