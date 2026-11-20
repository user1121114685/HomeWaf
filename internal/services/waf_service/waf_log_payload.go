// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package waf_service

import (
	"HomeWaf/internal/model"
	"HomeWaf/internal/model/innerbean"
	"HomeWaf/internal/platform/common/zlog"
	"HomeWaf/internal/storage/wafdb"
)

// payloadFetchChunk 一次 IN 查询带的 req_uuid 个数。
// SQLite 默认最多 999 个绑定变量，留足余量。
const payloadFetchChunk = 500

// FillLivePayloads 给实时库查出来的日志补上报文列。
func FillLivePayloads(rows []*innerbean.WebLog) {
	FillShardPayloads("", rows)
}

// FillShardPayloads 按 req_uuid 把报文补回日志对象。
func FillShardPayloads(currentDbName string, rows []*innerbean.WebLog) {
	if len(rows) == 0 {
		return
	}
	tier := wafdb.ResolveTierTables(currentDbName)
	if tier.Err != nil || tier.Payload == "" {
		return
	}

	index := make(map[string][]*innerbean.WebLog, len(rows))
	uuids := make([]string, 0, len(rows))
	for _, r := range rows {
		if r == nil || r.REQ_UUID == "" {
			continue
		}
		if _, ok := index[r.REQ_UUID]; !ok {
			uuids = append(uuids, r.REQ_UUID)
		}
		index[r.REQ_UUID] = append(index[r.REQ_UUID], r)
	}
	if len(uuids) == 0 {
		return
	}

	for start := 0; start < len(uuids); start += payloadFetchChunk {
		end := start + payloadFetchChunk
		if end > len(uuids) {
			end = len(uuids)
		}
		var found []model.EventPayload
		err := tier.DB.Table(tier.Payload).
			Select("req_uuid", "header", "cookies", "body", "res_body", "post_form", "res_header", "truncated").
			Where("req_uuid in ?", uuids[start:end]).Find(&found).Error
		if err != nil {
			// 报文读不到不该让整个页面失败：日志本身已经查出来了，报文列留空即可
			zlog.Warn("读取报文失败", "table", tier.Payload, "条数", end-start, "error", err.Error())
			return
		}
		for i := range found {
			p := &found[i]
			for _, r := range index[p.ReqUUID] {
				applyPayload(r, p)
			}
		}
	}
}

// Only the textual payload belongs in the API response. A truncated URL or payload marks the request as truncated.
func applyPayload(r *innerbean.WebLog, p *model.EventPayload) {
	r.HEADER = p.HEADER
	r.COOKIES = p.COOKIES
	r.BODY = p.BODY
	r.RES_BODY = p.RES_BODY
	r.POST_FORM = p.POST_FORM
	r.ResHeader = p.ResHeader
	if p.Truncated == 1 {
		r.Truncated = 1
	}
}
