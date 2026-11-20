package enums

const (
	// BATCHTASK_IPGROUP 导入到 IP 组。IP 组是租户级资源、不带 host_code，
	// 目标组由 batch_extra_config 里的 group_code 指定，与 batch_host_code 无关。
	BATCHTASK_IPGROUP                = "ipgroup"
	BATCHTASK_SENSITIVE              = "sensitive"
	BATCHTASK_EXECUTEMETHODAPPEND    = "append"
	BATCHTASK_EXECUTEMETHODOVERWRITE = "overwrite"

	BATCHTASK_SOURCETYPE_LOCAL  = "local"
	BATCHTASK_SOURCETYPE_REMOTE = "remote"

	// 触发类型
	BATCHTASK_TRIGGERTYPE_CRON   = "cron"
	BATCHTASK_TRIGGERTYPE_MANUAL = "manual"
)

// 以下四个白名单是批量任务的枚举边界。这些字段来自管理端提交、直接决定
// 「读哪里、写哪张表、怎么写」，不校验的后果不只是脏数据：未知的执行方式会让
// 各 processor 静默 return false（任务像成功一样什么都没做），未知的来源类型
// 会走进远端分支。所以保存时就拒，执行时再兜一次。

// IsValidBatchType 任务类型是否合法
func IsValidBatchType(v string) bool {
	switch v {
	case BATCHTASK_IPGROUP, BATCHTASK_SENSITIVE:
		return true
	}
	return false
}

// IsValidBatchSourceType 来源类型是否合法
func IsValidBatchSourceType(v string) bool {
	switch v {
	case BATCHTASK_SOURCETYPE_LOCAL, BATCHTASK_SOURCETYPE_REMOTE:
		return true
	}
	return false
}

// IsBatchLocalSource 是否为本地文件来源（非本地即远端拉取）
func IsBatchLocalSource(v string) bool {
	return v == BATCHTASK_SOURCETYPE_LOCAL
}

// IsValidBatchTriggerType 触发类型是否合法
func IsValidBatchTriggerType(v string) bool {
	switch v {
	case BATCHTASK_TRIGGERTYPE_CRON, BATCHTASK_TRIGGERTYPE_MANUAL:
		return true
	}
	return false
}

// IsValidBatchExecuteMethod 执行方式是否合法
func IsValidBatchExecuteMethod(v string) bool {
	switch v {
	case BATCHTASK_EXECUTEMETHODAPPEND, BATCHTASK_EXECUTEMETHODOVERWRITE:
		return true
	}
	return false
}
