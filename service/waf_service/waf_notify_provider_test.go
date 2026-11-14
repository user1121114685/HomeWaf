package waf_service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"HomeWaf/global"
	"HomeWaf/innerbean"
	"HomeWaf/model"
)

func TestNotifySubscriptionDeliversNewProviderWithTemplate(t *testing.T) {
	old := global.CurrentSettings().GCONFIG_OUTBOUND_ALLOWED_HOSTS
	global.UpdateSettings(func(c *global.RuntimeSettings) { c.GCONFIG_OUTBOUND_ALLOWED_HOSTS = "127.0.0.1" })
	t.Cleanup(func() {
		global.UpdateSettings(func(c *global.RuntimeSettings) { c.GCONFIG_OUTBOUND_ALLOWED_HOSTS = old })
	})
	var received map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Error(err)
		}
		fmt.Fprint(w, `{"code":200}`)
	}))
	defer server.Close()
	config, _ := json.Marshal(map[string]interface{}{"server": server.URL + "/device-key"})
	channel := model.NotifyChannel{Type: "bark", ConfigJSON: string(config)}
	sub := model.NotifySubscription{MessageType: model.MSG_TYPE_SYSTEM_ERROR, TitleTemplate: "告警：{{.ErrorType}}", ContentTemplate: "{{.ErrorMsg}}"}
	event := WafNotifySenderServiceApp.BuildNotifyEvent(innerbean.SystemErrorMessageInfo{ErrorType: "pipeline_lockdown", ErrorMsg: "[blocked] a_b *detail* \"quoted\""})
	title, content, used := RenderNotifyMessage(sub, channel.Type, event)
	if used != model.TemplateUsedCustom {
		t.Fatal("custom template not used")
	}
	if _, err := WafNotifySenderServiceApp.deliverToChannel(channel, sub, event.MessageType, title, content); err != nil {
		t.Fatal(err)
	}
	if received["title"] != "告警：pipeline_lockdown" || received["body"] != "[blocked] a_b *detail* \"quoted\"" {
		t.Fatalf("template text was corrupted: %#v", received)
	}
}
