// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

//go:build crossdb

// 静态数据加密（每实例 DEK）的跨引擎回归：SQLite / MySQL / PostgreSQL 各跑一遍。
//
package waf_service

import (
	"testing"

	"HomeWaf/model/request"
	"HomeWaf/wafdb/dialect"
	"HomeWaf/wafsec"

	"gorm.io/gorm"
)

func runDataKeyCases(t *testing.T, db *gorm.DB) {
	if !wafsec.DataKeyReady() {
		t.Fatalf("DEK 未就绪，跨库用例无法验证加密路径")
	}

	// —— 1. 密钥类配置项：加密落库 / 解密读出 / 非密钥项不受影响 ——
	t.Run("SystemConfigSecret", func(t *testing.T) {
		item := "gpt_token" // 敏感项
		secret := "sk-cross-" + sfx()
		svc := WafSystemConfigServiceApp

		// 清掉可能存在的同名行，保证 AddApi 不被幂等保护跳过
		db.Exec("DELETE FROM system_configs WHERE item = ?", item)
		fatalIf(t, svc.AddApi(request.WafSystemConfigAddReq{
			ItemClass: "gpt", Item: item, Value: secret, ItemType: "string",
		}))

		var raw string
		fatalIf(t, db.Raw("SELECT "+qc("value")+" FROM system_configs WHERE item = ?", item).Scan(&raw).Error)
		if raw == secret {
			t.Fatalf("密钥类配置不应明文落库")
		}
		if !wafsec.IsDataKeyCiphertext(raw) {
			t.Fatalf("落库值应为 swk1 密文，实际 %q", raw)
		}
		if got := svc.GetDetailByItem(item).Value; got != secret {
			t.Fatalf("读出应为明文: got %q want %q", got, secret)
		}
		if got := svc.GetAllConfigs()[item].Value; got != secret {
			t.Fatalf("GetAllConfigs 应为明文: got %q want %q", got, secret)
		}

		// 非密钥项保持明文，行为不变
		plainItem := "gpt_model"
		plainVal := "model-" + sfx()
		db.Exec("DELETE FROM system_configs WHERE item = ?", plainItem)
		fatalIf(t, svc.AddApi(request.WafSystemConfigAddReq{
			ItemClass: "gpt", Item: plainItem, Value: plainVal, ItemType: "string",
		}))
		var plainRaw string
		db.Raw("SELECT "+qc("value")+" FROM system_configs WHERE item = ?", plainItem).Scan(&plainRaw)
		if plainRaw != plainVal {
			t.Fatalf("非密钥项应明文落库: got %q want %q", plainRaw, plainVal)
		}
	})

}

// qc 按当前方言引用标识符（value / item 等在部分方言里是关键字）。
func qc(ident string) string { return dialect.Q(ident) }

// readSecretCol 读单个密文列（列名过方言引用，避开关键字问题）。
func readSecretCol(t *testing.T, db *gorm.DB, table, col, id string) string {
	t.Helper()
	var v string
	if err := db.Raw("SELECT "+qc(col)+" FROM "+qc(table)+" WHERE id = ?", id).Scan(&v).Error; err != nil {
		t.Fatalf("读取 %s.%s: %v", table, col, err)
	}
	return v
}
