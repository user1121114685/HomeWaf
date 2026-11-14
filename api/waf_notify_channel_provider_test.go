package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"HomeWaf/global"
	sqlite "HomeWaf/internal/sqlitedialect"
	"HomeWaf/model"
	"HomeWaf/model/common/response"
	"HomeWaf/wafsec"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestNotifyProviderCreateEditTestAndSecretStorage(t *testing.T) {
	if err := wafsec.InitDataKey(t.TempDir(), ""); err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(sqlite.Open(sqlite.FileDSN(filepath.Join(t.TempDir(), "notify.db"))), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if err = db.AutoMigrate(&model.NotifyChannel{}); err != nil {
		t.Fatal(err)
	}
	oldDB := global.GWAF_LOCAL_DB
	oldAllow := global.CurrentSettings().GCONFIG_OUTBOUND_ALLOWED_HOSTS
	global.GWAF_LOCAL_DB = db
	global.UpdateSettings(func(c *global.RuntimeSettings) { c.GCONFIG_OUTBOUND_ALLOWED_HOSTS = "127.0.0.1" })
	t.Cleanup(func() {
		global.GWAF_LOCAL_DB = oldDB
		global.UpdateSettings(func(c *global.RuntimeSettings) { c.GCONFIG_OUTBOUND_ALLOWED_HOSTS = oldAllow })
	})
	requests := 0
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/private-device-token" {
			t.Error("stored push address lost")
		}
		var body map[string]interface{}
		if json.NewDecoder(r.Body).Decode(&body) != nil || !strings.Contains(fmt.Sprint(body["title"]), "测试通知") {
			t.Error("invalid test message")
		}
		fmt.Fprint(w, `{"code":200}`)
	}))
	defer endpoint.Close()
	api := &WafNotifyChannelApi{}
	invoke := func(handler gin.HandlerFunc, payload interface{}) response.Response {
		data, _ := json.Marshal(payload)
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Set("is_openapi", true)
		c.Request = httptest.NewRequest("POST", "/notify", bytes.NewReader(data))
		c.Request.Header.Set("Content-Type", "application/json")
		handler(c)
		var envelope response.Response
		if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		return envelope
	}
	config := map[string]interface{}{"server": endpoint.URL + "/private-device-token", "group": "initial"}
	encode := func(value interface{}) string { b, _ := json.Marshal(value); return string(b) }
	input := map[string]interface{}{"name": "Bark fixture", "type": "bark", "status": 1, "config_json": encode(config)}
	if got := invoke(api.AddApi, input); got.Code != 0 {
		t.Fatal(got.Msg)
	}
	var stored model.NotifyChannel
	if err = db.First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	var cipherText string
	if err = db.Table("notify_channels").Select("config_json").Row().Scan(&cipherText); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(cipherText, "private-device-token") {
		t.Fatal("config stored as plaintext")
	}
	list := invoke(api.GetListApi, map[string]interface{}{"pageIndex": 1, "pageSize": 20})
	if list.Code != 0 {
		t.Fatal(list.Msg)
	}
	if strings.Contains(encode(list), "private-device-token") {
		t.Fatal("list revealed device key")
	}
	rows := list.Data.(map[string]interface{})["list"].([]interface{})
	edit := rows[0].(map[string]interface{})
	var masked map[string]interface{}
	_ = json.Unmarshal([]byte(edit["config_json"].(string)), &masked)
	masked["group"] = "edited"
	edit["config_json"] = encode(masked)
	edit["status"] = 0
	if got := invoke(api.ModifyApi, edit); got.Code != 0 {
		t.Fatal("masked secret was validated before merging:", got.Msg)
	}
	var updated model.NotifyChannel
	if err = db.First(&updated).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(updated.ConfigJSON, "private-device-token") || !strings.Contains(updated.ConfigJSON, "edited") || updated.Status != 0 {
		t.Fatal("edit lost values")
	}
	if got := invoke(api.TestApi, map[string]string{"id": stored.Id}); got.Code != 0 {
		t.Fatal(got.Msg)
	}
	if requests != 1 {
		t.Fatal("test did not use saved Bark transport")
	}
	masked["server"] = "__SAMWAF_CLEAR__"
	edit["config_json"] = encode(masked)
	if got := invoke(api.ModifyApi, edit); got.Code == 0 {
		t.Fatal("required secret was cleared")
	}
	edit["type"] = "telegram"
	edit["config_json"] = `{"chatId":"123"}`
	if got := invoke(api.ModifyApi, edit); got.Code == 0 {
		t.Fatal("type change inherited unrelated credential")
	}
	edit["config_json"] = `{"chatId":"123","apiToken":"fresh-bot-token"}`
	if got := invoke(api.ModifyApi, edit); got.Code != 0 {
		t.Fatal(got.Msg)
	}
	if err = db.First(&updated).Error; err != nil {
		t.Fatal(err)
	}
	if updated.Type != "telegram" || strings.Contains(updated.ConfigJSON, "private-device-token") {
		t.Fatal("old provider secret survived type switch")
	}
	input["type"] = "unknown"
	input["name"] = "unknown"
	if got := invoke(api.AddApi, input); got.Code == 0 {
		t.Fatal("unknown channel accepted")
	}
}
