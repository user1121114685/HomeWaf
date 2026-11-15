package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"net/netip"
	"testing"

	"HomeWaf/model"
	"HomeWaf/model/request"
	"HomeWaf/wafpipeline"
	"github.com/gin-gonic/gin"
)

func TestRuleBuilderPreviewCompilesAndMatchesNativeService(t *testing.T) {
	info, err := json.Marshal(model.RuleInfo{RuleBase: model.RuleBase{RuleName: "Service rule", Salience: 10}, RuleAction: "deny", RuleCondition: model.RuleCondition{RelationSymbol: "&&", RelationDetail: []model.RelationDetail{{FactName: "MF", Attr: "URL", AttrType: "string", AttrJudge: "==", AttrVal: "/private", AttrVal2: "true"}}}})
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(request.WafRulePreViewReq{RuleCode: "1234", FormSource: "builder", RuleJson: string(info)})
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("is_openapi", true)
	c.Request = httptest.NewRequest("POST", "/", bytes.NewReader(payload))
	c.Request.Header.Set("Content-Type", "application/json")
	(&WafRuleAPi{}).FormatRuleApi(c)
	var response struct {
		Code int `json:"code"`
		Data struct {
			Source string `json:"rule_content"`
		} `json:"data"`
		Message string `json:"msg"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil || response.Code != 0 || response.Data.Source == "" {
		t.Fatalf("format response: %s %v", recorder.Body.String(), err)
	}
	config := wafpipeline.PortConfig{Schema: 1, Port: 15000, Protocol: "http", Upstream: "http://127.0.0.1:5000", Policy: wafpipeline.DefaultPolicy()}
	config.Policy.Gate.Enabled = false
	config.Policy.RuleSet.Scripts = []wafpipeline.ScriptRule{{ID: "1234", Source: response.Data.Source, Mode: wafpipeline.ModeBlock, Severity: wafpipeline.Medium}}
	runtime := &wafpipeline.Runtime{}
	result, err := runtime.Explain(&wafpipeline.Snapshot{Config: config}, httptest.NewRequest("GET", "http://fixture.test/private", nil), netip.MustParseAddr("203.0.113.10"))
	if err != nil || result.Verdict.Action != wafpipeline.Deny || result.Verdict.RuleID != "1234" {
		t.Fatalf("native preview: %+v %v", result, err)
	}
}
