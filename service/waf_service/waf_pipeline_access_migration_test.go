package waf_service

import (
	"HomeWaf/wafpipeline"
	"strings"
	"testing"
)

func TestNativeTokenDraftMustHaveCredentialBeforeActivation(t *testing.T) {
	s, _ := nativePortFixture(t)
	c := wafpipeline.PortConfig{Schema: 1, Port: 5401, Protocol: "http", Upstream: "http://127.0.0.1:8080", Policy: newServicePolicyFixture()}
	c.Policy.Gate.RequireToken = true
	c.Policy.Gate.TokenHeader = "X-HomeWaf-Token"
	var err error
	c, err = s.Save(PortSaveRequest{Config: c})
	if err != nil {
		t.Fatal(err)
	}
	c.ListenEnabled = true
	if _, err := s.Save(PortSaveRequest{Config: c}); err == nil || !strings.Contains(err.Error(), "服务令牌") {
		t.Fatal("token-less draft could activate")
	}
}
