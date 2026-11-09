// HomeWaf modification (2026-11-10): fresh-install schema; see NOTICE.

package wafdb

import (
	"HomeWaf/enums"
	"HomeWaf/model"
	"gorm.io/gorm"
)

func defaultTasks() []model.Task {
	return []model.Task{
		{TaskName: "每1秒执行qps清空", TaskUnit: enums.TASK_SECOND, TaskAt: "", TaskValue: 1, TaskMethod: enums.TASK_RUNTIME_QPS_CLEAN},
		{TaskName: "每1秒重置QPS数据", TaskUnit: enums.TASK_SECOND, TaskAt: "", TaskValue: 1, TaskMethod: enums.TASK_HOST_QPS_CLEAN},
		{TaskName: "每天10s进行一次统计", TaskUnit: enums.TASK_SECOND, TaskValue: 10, TaskAt: "", TaskMethod: enums.TASK_COUNTER},
		{TaskName: "每10s推送系统统计数据", TaskUnit: enums.TASK_SECOND, TaskValue: 10, TaskAt: "", TaskMethod: enums.TASK_STATS_PUSH},
		{TaskName: "每30s进行健康度检测", TaskUnit: enums.TASK_SECOND, TaskValue: 30, TaskAt: "", TaskMethod: enums.TASK_HEALTH},
		{TaskName: "每1分钟进行一次延迟信息提取", TaskUnit: enums.TASK_MIN, TaskValue: 10, TaskAt: "", TaskMethod: enums.TASK_DELAY_INFO},
		{TaskName: "每1分钟进行一次配置更新", TaskUnit: enums.TASK_MIN, TaskValue: 1, TaskAt: "", TaskMethod: enums.TASK_LOAD_CONFIG},
		{TaskName: "每5分钟进行CCWindows旧信息清除", TaskUnit: enums.TASK_MIN, TaskValue: 5, TaskAt: "", TaskMethod: enums.TASK_CLEAR_CC_WINDOWS},
		{TaskName: "每5分钟进行防火墙IP封禁规则清理", TaskUnit: enums.TASK_MIN, TaskValue: 5, TaskAt: "", TaskMethod: enums.TASK_FIREWALL_CLEAN_EXPIRED},
		{TaskName: "每天30分钟删除历史下载文件", TaskUnit: enums.TASK_MIN, TaskValue: 30, TaskAt: "", TaskMethod: enums.TASK_DELETE_HISTORY_DOWNLOAD_FILE},
		{TaskName: "每1小时进行一次微信提取accesstoken", TaskUnit: enums.TASK_HOUR, TaskValue: 1, TaskAt: "", TaskMethod: enums.TASK_REFLUSH_WECHAT_ACCESS_TOKEN},
		{TaskName: "每天凌晨3点进行数据归档操作", TaskUnit: enums.TASK_DAY, TaskValue: 1, TaskAt: "03:00", TaskMethod: enums.TASK_SHARE_DB},
		{TaskName: "每天早5点删除历史信息", TaskUnit: enums.TASK_DAY, TaskValue: 1, TaskAt: "05:00", TaskMethod: enums.TASK_DELETE_HISTORY_INFO},
		{TaskName: "每天01:00进行GC回收", TaskUnit: enums.TASK_DAY, TaskValue: 1, TaskAt: "01:00", TaskMethod: enums.TASK_GC},
		{TaskName: "每天凌晨01:20进行数据库监控", TaskUnit: enums.TASK_DAY, TaskValue: 1, TaskAt: "01:20", TaskMethod: enums.TASK_DB_MONITOR},
		{TaskName: "每天02:13进行SSL证书申请", TaskUnit: enums.TASK_DAY, TaskValue: 1, TaskAt: "02:13", TaskMethod: enums.TASK_SSL_ORDER_RENEW},
		{TaskName: "每天03:00进行查询SSL绑定路径自动加载最新证书", TaskUnit: enums.TASK_DAY, TaskValue: 1, TaskAt: "03:00", TaskMethod: enums.TASK_SSL_PATH_LOAD},
		{TaskName: "每天05:00进行批量任务", TaskUnit: enums.TASK_DAY, TaskValue: 1, TaskAt: "05:00", TaskMethod: enums.TASK_BATCH},
		{TaskName: "每天06:00进行批量SSL过期检测", TaskUnit: enums.TASK_DAY, TaskValue: 1, TaskAt: "06:00", TaskMethod: enums.TASK_SSL_EXPIRE_CHECK},
		{TaskName: "每天早8晚8进行通知(需要开启通知)", TaskUnit: enums.TASK_DAY, TaskValue: 1, TaskAt: "08:00;20:00", TaskMethod: enums.TASK_NOTICE},
		{TaskName: "每周六晚上23:00进行web文件缓存清除", TaskUnit: enums.TASK_WEEKLY, TaskValue: 1, TaskAt: "23:00", TaskDaysOfTheWeek: "6", TaskMethod: enums.TASK_CLEAR_WEBCACHE},
		{TaskName: "每天凌晨04:30清理统计数据（按保留策略）", TaskUnit: enums.TASK_DAY, TaskValue: 1, TaskAt: "04:30", TaskMethod: enums.TASK_STATS_DATA_CLEANUP},
		{TaskName: "每5分钟按文件大小检测分库", TaskUnit: enums.TASK_MIN, TaskValue: 5, TaskAt: "", TaskMethod: enums.TASK_SHARE_DB_CHECK},
		{TaskName: "每1小时检查威胁情报IP订阅是否到期", TaskUnit: enums.TASK_HOUR, TaskValue: 1, TaskAt: "", TaskMethod: enums.TASK_THREAT_IP_SYNC},
		{TaskName: "每10分钟清理统一访问认证的过期会话/令牌/票据与审计日志", TaskUnit: enums.TASK_MIN, TaskValue: 10, TaskAt: "", TaskMethod: enums.TASK_ACCESS_CLEAN},
		{TaskName: "每10分钟清理网站密码访问的到期会话与历史记录", TaskUnit: enums.TASK_MIN, TaskValue: 10, TaskAt: "", TaskMethod: enums.TASK_HTTPAUTH_CLEAN},
		{TaskName: "每30秒把站点流量计量落库", TaskUnit: enums.TASK_SECOND, TaskValue: 30, TaskAt: "", TaskMethod: enums.TASK_TRAFFIC_FLUSH},
		{TaskName: "每1分钟解封主机防爆破中已到期的IP", TaskUnit: enums.TASK_MIN, TaskValue: 1, TaskAt: "", TaskMethod: enums.TASK_HOSTGUARD_CLEAN_EXPIRED},
		{TaskName: "每天05:30检测管理端证书（本地证书自动续期）", TaskUnit: enums.TASK_DAY, TaskValue: 1, TaskAt: "05:30", TaskMethod: enums.TASK_MANAGER_CERT_CHECK},
	}
}

// InitializeTasks seeds missing defaults and preserves user-edited schedules.
func InitializeTasks(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		for _, task := range defaultTasks() {
			task.BaseOrm = defaultBaseOrm()
			if err := tx.Where("task_method = ?", task.TaskMethod).Attrs(task).FirstOrCreate(&model.Task{}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
