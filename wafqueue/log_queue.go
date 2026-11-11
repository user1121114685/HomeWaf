// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package wafqueue

import (
	"HomeWaf/common/zlog"
	"HomeWaf/global"
	"HomeWaf/innerbean"
	"HomeWaf/model"
	"HomeWaf/wafipban"
	"HomeWaf/waftask"
	"strconv"
	"time"
)

/*
*
处理Log队列信息
*/
func ProcessLogDequeEngine() {
	for {
		select {
		case <-global.GWAF_QUEUE_SHUTDOWN_SIGNAL:
			zlog.Info("日志队列处理协程收到关闭信号，正在退出...")
			return
		default:
			global.GWAF_MEASURE_PROCESS_DEQUEENGINE.WriteData(time.Now().UnixNano() / 1e6)
			if global.GDATA_CURRENT_CHANGE {
				//如果正在切换库 跳过
				zlog.Debug("正在切换数据库等待中队列")

			} else {
				var webLogArray []*innerbean.WebLog
				var pipelineLogs []*model.PipelineLog
				var audits []*model.SecurityAuditLog
				batchCount := 0
				batchLimit := min(500, max(1, int(global.CurrentSettings().GDATA_BATCH_INSERT)))
				for !global.GQEQUE_LOG_DB.Empty() {
					global.IncrementLogQPS() // 使用统一的日志QPS增量函数
					weblogbean, ok := global.GQEQUE_LOG_DB.Dequeue()
					if !ok {
						continue
					}
					if weblogbean != nil {
						batchCount++
						// 进行类型断言将其转为具体的结构
						if logValue, ok := weblogbean.(*innerbean.WebLog); ok {
							webLogArray = append(webLogArray, logValue)
						} else {
							switch row := weblogbean.(type) {
							case *model.PipelineLog:
								pipelineLogs = append(pipelineLogs, row)
							case *model.SecurityAuditLog:
								audits = append(audits, row)
							default:
								global.GWAF_LOCAL_LOG_DB.Create(weblogbean)
							}
						}
					}
					if batchCount >= batchLimit {
						break
					}
				}
				if global.GWAF_LOCAL_LOG_DB != nil {
					if len(pipelineLogs) > 0 {
						if err := global.GWAF_LOCAL_LOG_DB.CreateInBatches(pipelineLogs, 100).Error; err != nil {
							zlog.Error("Pipeline log batch failed", err.Error())
						}
					}
					if len(audits) > 0 {
						if err := global.GWAF_LOCAL_LOG_DB.CreateInBatches(audits, 100).Error; err != nil {
							zlog.Error("Security audit batch failed", err.Error())
						}
					}
				}
				if len(webLogArray) > 0 {
					zlog.Debug("日志队列处理协程处理日志数量:" + strconv.Itoa(len(webLogArray)))
					// 检查失败状态码并记录IP失败
					if global.CurrentSettings().GCONFIG_IP_FAILURE_BAN_ENABLED == 1 {
						ipManager := wafipban.GetIPFailureManager()
						for _, log := range webLogArray {
							if ipManager.IsFailureStatusCode(log.STATUS_CODE) {
								ipManager.RecordFailure(log)
							}
						}
					}
					if global.CurrentSettings().GCONFIG_LOG_PERSIST_ENABLED == 1 {
						// 分层落库：安全事件进 security_event（带报文），窄行进 access_log（按档位），
						// web_logs 不再写入（旧数据只读到期删）。
						// 全部在副本上动手，下面的统计与出口仍拿完整的原始对象。
						storeTiered(webLogArray)
						// 蓄水池采到的正常请求报文攒在内存里，到点整池刷成 event_payload(kind=sample)
						negSampler.FlushIfDue()
					}
					// 日志流做统计
					waftask.CollectStatsFromLogs(webLogArray)
					// 分析层天级汇总（行为 / 目标 / 手法三视角）。键在本包算好再传过去：
					// waftask 不能反向 import 本包，而键必须与窄行同源。
					waftask.CollectAnalysisStats(AnalysisRowsFromLogs(webLogArray))
					global.GNOTIFY_KAKFA_SERVICE.ProcessBatchLogs(webLogArray)
					// 文件日志写入
					global.GNOTIFY_LOG_FILE_WRITER.ProcessBatchLogs(webLogArray)
				}
			}
			time.Sleep(100 * time.Millisecond)
			global.GWAF_MEASURE_PROCESS_DEQUEENGINE.WriteData(time.Now().UnixNano() / 1e6)
		}
	}
}
