package waf_service

import (
	"HomeWaf/model"
	"HomeWaf/model/request"
	"HomeWaf/wafenginecore/ipset"
	"HomeWaf/wafpipeline"
	"encoding/json"
	"testing"
)

func TestIPGroupInitialAddressesAreAtomicAndDualStack(t *testing.T) {
	db := setupIPGroupTestDB(t)
	service := WafIPGroupServiceApp
	if _, err := service.AddApi(request.WafIPGroupAddReq{GroupName: "invalid", InitialIPs: []string{"192.168.0.0/16", "300.1.1.1"}}); err == nil {
		t.Fatal("invalid initial addresses accepted")
	}
	for _, value := range []string{"*.*.*.*", "*:*:*:*:*:*:*:*", "192.168.1.9-192.168.1.1"} {
		if _, err := service.AddApi(request.WafIPGroupAddReq{GroupName: "invalid", InitialIPs: []string{value}}); err == nil {
			t.Fatalf("initial addresses bypassed existing IP group validation: %s", value)
		}
	}
	var count int64
	db.Model(&model.IPGroup{}).Count(&count)
	if count != 0 {
		t.Fatal("invalid group partially saved")
	}
	group, err := service.AddApi(request.WafIPGroupAddReq{GroupName: "内网", InitialIPs: []string{"192.168.0.0/16", "fd00::/8", "192.168.0.0/16"}})
	if err != nil || group.ItemCount != 2 {
		t.Fatalf("%+v %v", group, err)
	}
	service.RebuildGroupMatcher(group.GroupCode)
	if !ipset.GetGroupMatcher(group.GroupCode).ContainsStr("192.168.1.1") || !ipset.GetGroupMatcher(group.GroupCode).ContainsStr("fd00::1") {
		t.Fatal("created group not available to shared matcher")
	}
}

func TestPipelineGroupReferencesPreventDeletionAndValidateMissing(t *testing.T) {
	db := setupIPGroupTestDB(t)
	if err := db.AutoMigrate(&model.PortConfig{}); err != nil {
		t.Fatal(err)
	}
	group := mustAddGroup(t, "家庭")
	config := wafpipeline.PortConfig{Name: "相册", Port: 5000, Policy: wafpipeline.DefaultPolicy()}
	config.Policy.IPGroup.AllowGroups = []string{group.GroupCode}
	config.Policy.IPGroup.DenyGroups = []string{group.GroupCode}
	data, _ := json.Marshal(config)
	if err := db.Table("port_config").Create(map[string]any{"port": 5000, "config_json": string(data)}).Error; err != nil {
		t.Fatal(err)
	}
	refs := WafIPGroupServiceApp.GetRefsApi(group.GroupCode)
	if len(refs.Ports) != 1 || refs.Ports[0].Name != "相册" || !refs.Ports[0].Allow || !refs.Ports[0].Block {
		t.Fatalf("missing references: %+v", refs)
	}
	for _, force := range []int{0, 1} {
		if _, err := WafIPGroupServiceApp.DelApi(request.WafIPGroupDelReq{Id: group.Id, Force: force}); err == nil {
			t.Fatal("referenced group deleted")
		}
	}
	if err := validatePipelineGroups(db, config.Policy.IPGroup); err != nil {
		t.Fatal(err)
	}
	config.Policy.IPGroup.AllowGroups = []string{"missing"}
	if err := validatePipelineGroups(db, config.Policy.IPGroup); err == nil {
		t.Fatal("missing group accepted")
	}
	if err := db.Where("port = ?", 5000).Delete(&model.PortConfig{}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := WafIPGroupServiceApp.DelApi(request.WafIPGroupDelReq{Id: group.Id}); err != nil {
		t.Fatal(err)
	}
}
