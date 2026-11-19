package enums

import "testing"

// 批量任务枚举白名单用例。这四个字段来自管理端提交，未知值以前会一路走到执行侧，
// 表现为"任务像成功一样什么都没做"或"走进远端拉取分支"。

func TestIsValidBatchType(t *testing.T) {
	for _, v := range []string{BATCHTASK_IPGROUP, BATCHTASK_SENSITIVE} {
		if !IsValidBatchType(v) {
			t.Fatalf("%q 应为合法任务类型", v)
		}
	}
	for _, v := range []string{"", " ", "IPDENY", "ipdeny", "ipdeny ", "ipallow", "rule", "../etc", "sensitive;drop"} {
		if IsValidBatchType(v) {
			t.Fatalf("%q 必须被拒", v)
		}
	}
}

func TestIsValidBatchSourceType(t *testing.T) {
	for _, v := range []string{BATCHTASK_SOURCETYPE_LOCAL, BATCHTASK_SOURCETYPE_REMOTE} {
		if !IsValidBatchSourceType(v) {
			t.Fatalf("%q 应为合法来源类型", v)
		}
	}
	for _, v := range []string{"", "LOCAL", "url", "file", "ftp", "local "} {
		if IsValidBatchSourceType(v) {
			t.Fatalf("%q 必须被拒", v)
		}
	}
}

func TestIsBatchLocalSource(t *testing.T) {
	if !IsBatchLocalSource(BATCHTASK_SOURCETYPE_LOCAL) {
		t.Fatal("local 应判定为本地来源")
	}
	for _, v := range []string{BATCHTASK_SOURCETYPE_REMOTE, "", "LOCAL"} {
		if IsBatchLocalSource(v) {
			t.Fatalf("%q 不应判定为本地来源", v)
		}
	}
}

func TestIsValidBatchTriggerType(t *testing.T) {
	for _, v := range []string{BATCHTASK_TRIGGERTYPE_CRON, BATCHTASK_TRIGGERTYPE_MANUAL} {
		if !IsValidBatchTriggerType(v) {
			t.Fatalf("%q 应为合法触发类型", v)
		}
	}
	for _, v := range []string{"", "CRON", "auto", "cron "} {
		if IsValidBatchTriggerType(v) {
			t.Fatalf("%q 必须被拒", v)
		}
	}
}

func TestIsValidBatchExecuteMethod(t *testing.T) {
	for _, v := range []string{BATCHTASK_EXECUTEMETHODAPPEND, BATCHTASK_EXECUTEMETHODOVERWRITE} {
		if !IsValidBatchExecuteMethod(v) {
			t.Fatalf("%q 应为合法执行方式", v)
		}
	}
	for _, v := range []string{"", "APPEND", "sync", "replace", "append "} {
		if IsValidBatchExecuteMethod(v) {
			t.Fatalf("%q 必须被拒", v)
		}
	}
}
