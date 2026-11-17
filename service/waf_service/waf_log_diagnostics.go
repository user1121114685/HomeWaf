package waf_service

import (
	"HomeWaf/global"
	"HomeWaf/innerbean"
	"HomeWaf/wafpipeline"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

var diagnosticSignature = regexp.MustCompile(`^([a-z_]+) signature in ([a-z_]+) \(([a-z_]+)\)$`)

func requestDiagnostics(log *innerbean.WebLog) *innerbean.RequestDiagnostics {
	out := &innerbean.RequestDiagnostics{State: "unavailable", Hits: []innerbean.DiagnosticHit{}}
	// The caller first resolves the request through the tenant-scoped log lookup.
	// Pipeline rows have no tenant columns; never expose them through an arbitrary
	// request-ID lookup without that authorized parent record.
	if log.TenantId != global.GWAF_TENANT_ID || log.USER_CODE != global.GWAF_USER_CODE || log.REQ_UUID == "" || global.GWAF_LOCAL_LOG_DB == nil {
		return out
	}
	var row struct{ DetailJSON, PortID string }
	query := "SELECT detail_json, port_id FROM pipeline_log WHERE req_id = ? AND kind = ? AND client_ip = ?"
	args := []any{log.REQ_UUID, "request", log.SRC_IP}
	if port, err := strconv.ParseUint(strings.TrimPrefix(log.HOST_CODE, "pipeline-"), 10, 16); strings.HasPrefix(log.HOST_CODE, "pipeline-") && err == nil {
		query += " AND port_id = ?"
		args = append(args, wafpipeline.PortID(uint16(port)))
	}
	// Pipeline traces remain in the live log store, even when the request's
	// access/security row has moved into an archive shard.
	result := global.GWAF_LOCAL_LOG_DB.Raw(query+" ORDER BY time_millis DESC, id DESC LIMIT 1", args...).Scan(&row)
	if result.Error != nil {
		return out
	}
	if result.RowsAffected == 0 {
		out.State = "missing"
		return out
	}
	var detail wafpipeline.Result
	if len(row.DetailJSON) > 1<<20 || json.Unmarshal([]byte(row.DetailJSON), &detail) != nil || detail.ReqID != log.REQ_UUID {
		out.State = "invalid"
		return out
	}
	if len(detail.Hits) > 128 {
		out.State = "invalid"
		return out
	}
	for _, hit := range detail.Hits {
		if hit.ReqID != log.REQ_UUID || hit.PortID != row.PortID {
			out.State = "invalid"
			out.Hits = []innerbean.DiagnosticHit{}
			return out
		}
		if hit.Category == "auth_success" || hit.Category == "business_auth_success" {
			continue
		}
		item := innerbean.DiagnosticHit{Category: hit.Category, RuleID: hit.RuleID, Severity: string(hit.Severity), Observe: hit.Observe, Trusted: hit.Trusted}
		// Parse only the detector-owned signature format. Do not forward raw
		// evidence, which other detectors can populate with request-derived data.
		if parts := diagnosticSignature.FindStringSubmatch(hit.Evidence); len(parts) == 4 && parts[1] == hit.Category {
			item.Source, item.Code = parts[2], parts[3]
		}
		out.Hits = append(out.Hits, item)
	}
	out.State, out.BodyTruncated = "available", detail.BodyTruncated
	return out
}
