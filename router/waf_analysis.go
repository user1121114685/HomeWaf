// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package router

import (
	"HomeWaf/api"
	"github.com/gin-gonic/gin"
)

type AnalysisRouter struct {
}

func (receiver *AnalysisRouter) InitAnalysisRouter(group *gin.RouterGroup) {
	analysisApi := api.APIGroupAPP.WafAnalysisApi
	router := group.Group("")
	//数据分析
	router.GET("/api/v1/analysis/wafanalysisdaycountryrange", analysisApi.StatAnalysisDayCountryRangeApi)
	router.GET("/api/v1/analysis/spider", analysisApi.AnalysisSpiderRangeApi)
	// 来源与路径分析（M5/G3）：两个视角 + 抽屉下钻，只读
	router.GET("/api/v1/analysis/actor/list", analysisApi.AnalysisActorListApi)
	router.GET("/api/v1/analysis/path/list", analysisApi.AnalysisPathListApi)
	router.GET("/api/v1/analysis/detail", analysisApi.AnalysisDetailApi)
}
