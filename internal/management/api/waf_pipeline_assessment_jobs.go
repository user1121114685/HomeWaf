package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"HomeWaf/internal/model/common/response"
	"HomeWaf/internal/protection/wafpipeline"
	"github.com/gin-gonic/gin"
)

type assessmentJob struct {
	ID       string `json:"id"`
	State    string `json:"state"`
	Done     int    `json:"done"`
	Total    int    `json:"total"`
	Error    string `json:"error,omitempty"`
	Result   gin.H  `json:"result,omitempty"`
	owner    string
	cancel   context.CancelFunc
	finished time.Time
}

var assessmentJobs = struct {
	sync.Mutex
	items map[string]*assessmentJob
}{items: map[string]*assessmentJob{}}

func assessmentOwner(c *gin.Context) string {
	if key := c.GetString("openapi_key_id"); key != "" {
		return "key:" + key
	}
	if account := pipelineOperator(c); account != "" {
		return "account:" + account
	}
	return ""
}
func startAssessmentJob(c *gin.Context, input PipelineAssessmentRequest, snapshot *wafpipeline.Snapshot) {
	owner := assessmentOwner(c)
	if owner == "" {
		response.FailWithMessage("无法确认任务所属账号，请重新登录", c)
		return
	}
	select {
	case assessmentSlots <- struct{}{}:
	default:
		response.FailWithMessage("检测任务已满，请稍后重试", c)
		return
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		<-assessmentSlots
		response.FailWithMessage("无法创建检测任务", c)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	job := &assessmentJob{ID: hex.EncodeToString(nonce[:]), State: "preparing", owner: owner, cancel: cancel}
	assessmentJobs.Lock()
	for id, old := range assessmentJobs.items {
		if !old.finished.IsZero() && time.Since(old.finished) > 10*time.Minute {
			delete(assessmentJobs.items, id)
		}
	}
	for len(assessmentJobs.items) >= 8 {
		oldest := ""
		var at time.Time
		for id, old := range assessmentJobs.items {
			if !old.finished.IsZero() && (oldest == "" || old.finished.Before(at)) {
				oldest, at = id, old.finished
			}
		}
		if oldest == "" {
			break
		}
		delete(assessmentJobs.items, oldest)
	}
	assessmentJobs.items[job.ID] = job
	assessmentJobs.Unlock()
	response.OkWithDetailed(gin.H{"job": gin.H{"id": job.ID, "state": "preparing", "done": 0, "total": 0}}, "检测已开始", c)
	go func() {
		defer func() { cancel(); <-assessmentSlots }()
		defer func() {
			time.AfterFunc(10*time.Minute, func() { assessmentJobs.Lock(); delete(assessmentJobs.items, job.ID); assessmentJobs.Unlock() })
		}()
		defer func() {
			if recover() != nil {
				assessmentJobs.Lock()
				job.State = "failed"
				job.Error = "检测执行异常，请重试"
				job.finished = time.Now()
				assessmentJobs.Unlock()
			}
		}()
		result, err := executeAssessment(ctx, input, snapshot, func(done, total int) {
			assessmentJobs.Lock()
			job.State = "running"
			job.Done = done
			job.Total = total
			assessmentJobs.Unlock()
		})
		assessmentJobs.Lock()
		defer assessmentJobs.Unlock()
		job.finished = time.Now()
		if err == nil {
			err = ctx.Err()
		}
		if err != nil {
			job.State = "failed"
			job.Error = err.Error()
			if errors.Is(err, context.Canceled) {
				job.State = "cancelled"
				job.Error = "检测已取消"
			}
			if errors.Is(err, context.DeadlineExceeded) {
				job.Error = "检测超时，请减少自定义样本或重试"
			}
			return
		}
		job.State = "complete"
		job.Result = result
	}()
}
func (w *WafPipelineApi) AssessmentJob(c *gin.Context) {
	assessmentJobs.Lock()
	defer assessmentJobs.Unlock()
	job := assessmentJobs.items[c.Query("id")]
	if job == nil || job.owner != assessmentOwner(c) || (!job.finished.IsZero() && time.Since(job.finished) > 10*time.Minute) {
		response.FailWithMessage("检测任务不存在或已过期，请重新开始", c)
		return
	}
	response.OkWithDetailed(job, "", c)
}
func (w *WafPipelineApi) CancelAssessment(c *gin.Context) {
	var input struct {
		ID string `json:"id"`
	}
	if !pipelineBind(c, &input) {
		return
	}
	assessmentJobs.Lock()
	defer assessmentJobs.Unlock()
	job := assessmentJobs.items[input.ID]
	if job == nil || job.owner != assessmentOwner(c) {
		response.FailWithMessage("检测任务不存在或已过期", c)
		return
	}
	if job.finished.IsZero() {
		job.cancel()
	}
	response.OkWithMessage("已请求取消检测", c)
}
