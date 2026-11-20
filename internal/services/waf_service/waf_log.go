// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package waf_service

import (
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/model/innerbean"
	"HomeWaf/internal/model/request"
	"HomeWaf/internal/platform/common/zlog"
	"HomeWaf/internal/storage/wafdb"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type WafLogService struct{}

var WafLogServiceApp = new(WafLogService)

var ErrLogNotFound = errors.New("未找到该识别码的日志，记录可能未保存或已过保留期")

func (receiver *WafLogService) GetDetailApi(req request.WafAttackLogDetailReq) (innerbean.WebLog, error) {
	var weblog innerbean.WebLog
	req.REQ_UUID = strings.TrimSpace(req.REQ_UUID)
	if req.REQ_UUID == "" {
		return weblog, ErrLogNotFound
	}
	// 事件双写了窄行，内容一致，但事件表保留期更长，优先从它读。
	// 分区标识为空或 auto 时先按识别码定位分区：详情最常见的来路就是
	// 用户手里只有一串识别码，不知道那次访问落在哪个分区
	shardName := ResolveDetailShard(req.CurrrentDbName, req.REQ_UUID)
	tier := wafdb.ResolveTierTables(shardName)
	if tier.Err != nil {
		if errors.Is(tier.Err, wafdb.ErrShardMissing) {
			return weblog, fmt.Errorf("该分区的存储已不存在，可在「分区管理」里删除这条登记")
		}
		return weblog, fmt.Errorf("打开日志分区失败: %w", tier.Err)
	}
	found := false
	for _, table := range []string{tier.Event, tier.Access} {
		if table == "" {
			continue
		}
		res := tier.DB.Table(table).Select("*").Where("REQ_UUID=?", req.REQ_UUID).Find(&weblog)
		if res.Error != nil {
			return weblog, fmt.Errorf("查询日志详情失败: %w", res.Error)
		}
		if res.RowsAffected > 0 {
			found = true
			break
		}
	}
	if !found {
		return weblog, ErrLogNotFound
	}
	// 告诉界面这条是在哪个分区找到的：auto 时用户并没有选分区，得有个地方说清楚
	weblog.ShardName = shardName
	if weblog.ShardName == "" {
		weblog.ShardName = wafdb.LiveLogName()
	}
	// 报文单独存在 event_payload 里，按主键点查补回来；没有报文行的（正常请求未采样）保持窄字段
	FillShardPayloads(shardName, []*innerbean.WebLog{&weblog})
	weblog.Diagnostics = requestDiagnostics(&weblog)
	return weblog, nil
}

// DeleteHistory 分层保留期清理：
//   - security_event按「日志保留天数」删
//   - access_log 按 access_log_retention_days 删（更短）
//   - event_payload 跟属主走：kind=event 随安全事件，kind=sample 只留 30 天，
//     kind=watch（观察名单全量留痕）只留 watchPayloadRetentionDays 天
//
// 三张新表都带同格式的 create_time，按各自条件删，不必回表对 req_uuid。
func (receiver *WafLogService) DeleteHistory(securityDay, accessDay string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	release, err := wafdb.AcquireLogWrite(ctx)
	if err != nil {
		zlog.Debug("清理日志", "日志来源繁忙，本轮延后清理")
		return
	}
	defer release()
	global.GWAF_LOCAL_LOG_DB.Where("create_time < ?", securityDay).Delete(&model.SecurityEvent{})
	if err := global.GWAF_LOCAL_LOG_DB.Where("create_time < ?", accessDay).
		Delete(&model.AccessLog{}).Error; err != nil {
		zlog.Warn("清理过期访问日志失败", "截止", accessDay, "error", err.Error())
	}
	sampleDay := time.Now().AddDate(0, 0, -sampleRetentionDays).Format("2006-01-02 15:04")
	watchDay := time.Now().AddDate(0, 0, -watchPayloadRetentionDays).Format("2006-01-02 15:04")
	if err := global.GWAF_LOCAL_LOG_DB.
		Where("(kind = ? and create_time < ?) or (kind = ? and create_time < ?) or (kind = ? and create_time < ?)",
			"event", securityDay, "sample", sampleDay, "watch", watchDay).
		Delete(&model.EventPayload{}).Error; err != nil {
		zlog.Warn("清理过期报文失败", "截止", securityDay, "error", err.Error())
	}
}

// sampleRetentionDays 采样请求报文的保留天数，与访问窄行独立，用于请求回溯。
const sampleRetentionDays = 30

// watchPayloadRetentionDays 观察名单全量留痕报文的保留天数（D9）。窄行仍随 access_log 保留期。
const watchPayloadRetentionDays = 7

/*
*
判断是否合法
*/
func (receiver *WafLogService) isValidSortField(field string) bool {
	var allowedSortFields = []string{"time_spent", "create_time", "unix_add_time"}

	for _, allowedField := range allowedSortFields {
		if field == allowedField {
			return true
		}
	}
	return false
}
