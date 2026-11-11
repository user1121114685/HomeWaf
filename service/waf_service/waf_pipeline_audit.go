package waf_service

import (
	"HomeWaf/model"
	"HomeWaf/wafpipeline"
	"fmt"
	"reflect"
	"strings"
)

func portChangeSummary(old, next *wafpipeline.Snapshot) string {
	var previous wafpipeline.PortConfig
	if old != nil {
		previous = old.Config
	}
	before, after := reflect.ValueOf(previous), reflect.ValueOf(next.Config)
	var changed []string
	for i := 0; i < after.NumField(); i++ {
		field := after.Type().Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "revision" || name == "access_changed_at" || name == "access_changed_by" {
			continue
		}
		if field.Name == "Policy" {
			left, right := before.Field(i), after.Field(i)
			for j := 0; j < right.NumField(); j++ {
				if old == nil || !reflect.DeepEqual(left.Field(j).Interface(), right.Field(j).Interface()) {
					changed = append(changed, "policy."+strings.Split(right.Type().Field(j).Tag.Get("json"), ",")[0])
				}
			}
		} else if old == nil || !reflect.DeepEqual(before.Field(i).Interface(), after.Field(i).Interface()) {
			changed = append(changed, name)
		}
	}
	if old == nil || !reflect.DeepEqual(old.Secrets, next.Secrets) {
		changed = append(changed, "credentials")
	}
	return fmt.Sprintf("revision=%d; changed=%s", next.Config.Revision, strings.Join(changed, ","))
}
func (s *WafPipelineService) auditDetail(operator string, port uint16, event, detail string) {
	WafSecurityAuditServiceApp.Write(AuditEntry{Event: event, AccountName: operator, HostCode: wafpipeline.PortID(port), Message: detail, Result: model.AccessAuditOK})
}
