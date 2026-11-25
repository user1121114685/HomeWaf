package api

import (
	"HomeWaf/internal/model/common/response"
	"HomeWaf/internal/protection/wafpipeline"
	"HomeWaf/internal/services/waf_service"
	"context"
	"github.com/gin-gonic/gin"
	"time"
)

// AssessSingle is synchronous and does not create a retained job or score.
func (w *WafPipelineApi) AssessSingle(c *gin.Context) {
	var input struct {
		Port             uint16                     `json:"port"`
		ExpectedRevision *int64                     `json:"expected_revision"`
		Config           *wafpipeline.PortConfig    `json:"config,omitempty"`
		Request          *wafpipeline.SingleRequest `json:"request"`
		ProfileID        string                     `json:"profile_id,omitempty"`
	}
	if !pipelineBind(c, &input) {
		return
	}
	if input.Request == nil || input.ExpectedRevision == nil {
		response.FailWithMessage("需要一条请求和所选服务的配置版本", c)
		return
	}
	select {
	case assessmentSlots <- struct{}{}:
		defer func() { <-assessmentSlots }()
	default:
		response.FailWithMessage("检测任务已满，请稍后重试", c)
		return
	}
	snapshot, err := assessmentSnapshot(PipelineAssessmentRequest{Port: input.Port, ExpectedRevision: input.ExpectedRevision, Config: input.Config})
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	var profile *waf_service.AssessmentProfile
	if input.ProfileID != "" {
		rows := waf_service.AssessmentCamouflage(&wafpipeline.AssessmentReport{}, []wafpipeline.AssessmentSource{{IP: input.Request.SourceIP}}, input.ProfileID)
		for i := range rows {
			if rows[i].ID == input.ProfileID {
				profile = &rows[i]
				break
			}
		}
		if profile == nil {
			response.FailWithMessage("所选伪装画像不存在，请刷新画像后重试", c)
			return
		}
	}
	result, err := wafpipeline.Live.AssessSingle(ctx, snapshot, *input.Request)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	current, err := waf_service.WafPipelineServiceApp.Get(input.Port)
	if err != nil || current.Revision != snapshot.Config.Revision {
		response.FailWithMessage("检测期间服务版本已变化，请刷新配置后重试", c)
		return
	}
	response.OkWithDetailed(gin.H{"decision": result, "profile": profile}, "单条判定完成", c)
}
