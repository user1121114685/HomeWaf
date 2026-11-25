package api

import (
	"context"
	"errors"
	"time"

	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model/common/response"
	"HomeWaf/internal/protection/wafpipeline"
	"HomeWaf/internal/services/waf_service"
	"github.com/gin-gonic/gin"
)

type PipelineAssessmentRequest struct {
	Port             uint16                  `json:"port"`
	ExpectedRevision *int64                  `json:"expected_revision,omitempty"`
	Config           *wafpipeline.PortConfig `json:"config,omitempty"`
	wafpipeline.AssessmentInput
	ProfileID  string `json:"profile_id"`
	Automatic  bool   `json:"automatic"`
	Background bool   `json:"background"`
}

// Global admission bound keeps authenticated bulk simulations from starving protection.
var assessmentSlots = make(chan struct{}, 2)

func assessmentSnapshot(input PipelineAssessmentRequest) (*wafpipeline.Snapshot, error) {
	config, err := waf_service.WafPipelineServiceApp.Get(input.Port)
	if err != nil {
		return nil, err
	}
	if input.ExpectedRevision != nil && *input.ExpectedRevision != config.Revision {
		return nil, errors.New("服务版本已变化，请刷新服务配置后检测")
	}
	if input.Config != nil {
		if input.Config.Port != input.Port || input.Config.Revision != config.Revision {
			return nil, errors.New("服务或版本已变化，请重新读取配置后检测")
		}
		config = wafpipeline.CloneConfig(*input.Config)
	}
	snapshot := &wafpipeline.Snapshot{Config: config}
	if current, ok := wafpipeline.Ports.Get(input.Port); ok {
		snapshot.Secrets = current.Secrets
	}
	return snapshot, nil
}

func (w *WafPipelineApi) AssessmentSources(c *gin.Context) {
	select {
	case assessmentSlots <- struct{}{}:
		defer func() { <-assessmentSlots }()
	default:
		response.FailWithMessage("检测任务已满，请稍后重试", c)
		return
	}
	var input PipelineAssessmentRequest
	if !pipelineBind(c, &input) {
		return
	}
	snapshot, err := assessmentSnapshot(input)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	plan, err := waf_service.BuildAssessmentPlan(ctx, snapshot.Config)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithDetailed(plan, "", c)
}

func (w *WafPipelineApi) AssessPort(c *gin.Context) {
	var input PipelineAssessmentRequest
	if !pipelineBind(c, &input) {
		return
	}
	snapshot, err := assessmentSnapshot(input)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	if input.Automatic || input.Background {
		startAssessmentJob(c, input, snapshot)
		return
	}
	select {
	case assessmentSlots <- struct{}{}:
		defer func() { <-assessmentSlots }()
	default:
		response.FailWithMessage("检测任务已满，请稍后重试", c)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	result, err := executeAssessment(ctx, input, snapshot, nil)
	if err != nil {
		response.FailWithMessage(err.Error(), c)
		return
	}
	response.OkWithDetailed(result, "检测完成，服务配置未改变", c)
}
func executeAssessment(ctx context.Context, input PipelineAssessmentRequest, snapshot *wafpipeline.Snapshot, progress func(int, int)) (gin.H, error) {
	var plan *waf_service.AssessmentPlan
	if input.Automatic {
		generated, err := waf_service.BuildAssessmentPlan(ctx, snapshot.Config)
		if err != nil {
			return nil, err
		}
		plan = &generated
		input.AssessmentInput = wafpipeline.AssessmentInput{Samples: generated.Samples, Pressure: generated.Pressure, PressureMode: "mixed"}
		for _, source := range generated.Sources {
			input.Sources = append(input.Sources, wafpipeline.AssessmentSource{IP: source.IP, ExpectAccess: source.ExpectAccess})
		}
	}
	if !input.Automatic && input.ProfileID != "" {
		found := false
		for _, profile := range waf_service.WafCamouflageServiceApp.Config().Profiles {
			if profile.ID == input.ProfileID {
				found = true
			}
		}
		if !found {
			return nil, errors.New("伪装画像已变化，请重新读取")
		}
	}
	var report wafpipeline.AssessmentReport
	var err error
	if input.Automatic || input.Background {
		report, err = wafpipeline.Live.AssessMany(ctx, snapshot, input.AssessmentInput, progress)
	} else {
		report, err = wafpipeline.Live.Assess(ctx, snapshot, input.AssessmentInput)
	}
	if err != nil {
		return nil, err
	}
	current, err := waf_service.WafPipelineServiceApp.Get(input.Port)
	if err != nil || current.Revision != snapshot.Config.Revision {
		return nil, errors.New("检测期间服务版本已变化，请重新检测")
	}
	if plan != nil {
		if global.GIPLOCATION_MANAGER == nil || plan.Generation != global.GIPLOCATION_MANAGER.Generation() {
			return nil, errors.New("IP 数据库已更新，请重新检测")
		}
		report.Advice = append(report.Advice, plan.Warnings...)
	}
	profiles := waf_service.AssessmentCamouflage(&report, input.Sources, input.ProfileID)
	var listeners []waf_service.AssessmentListener
	if input.Automatic {
		listeners = waf_service.ProbeAssessmentListeners(ctx, current)
		for _, listener := range listeners {
			if listener.Connected && !listener.Expected {
				report.Advice = append(report.Advice, listener.Family+" 配置为关闭，但本机该端口仍可建立 TCP 连接，请检查实际监听。")
			}
		}
		report.Limits = append(report.Limits, "协议检查只对所选服务的本机端口建立 TCP 连接，不发送攻击报文；未连通可能由本机访问限制导致，不能证明公网不可达。草稿的监听变更需保存后复测。")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return gin.H{"assessment": report, "camouflage": profiles, "plan": plan, "listeners": listeners}, nil
}
