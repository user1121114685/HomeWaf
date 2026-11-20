package model

// EventPayload holds request and response content for an event, sampled request or watched IP.
// Lists and aggregates read narrow rows; detail queries load this table by request ID.
type EventPayload struct {
	ReqUUID  string `gorm:"column:req_uuid;size:64;primaryKey" json:"req_uuid"`
	TenantId string `gorm:"size:64" json:"tenant_id"`
	UserCode string `gorm:"size:64" json:"user_code"`
	HostCode string `gorm:"size:64" json:"host_code"`
	Kind     string `gorm:"size:16" json:"kind"`

	HEADER    string `gorm:"type:text" json:"header"`
	COOKIES   string `gorm:"type:text" json:"cookies"`
	BODY      string `gorm:"type:text" json:"body"`
	RES_BODY  string `gorm:"type:text" json:"res_body"`
	POST_FORM string `gorm:"type:text" json:"post_form"`
	ResHeader string `gorm:"type:text" json:"res_header"`

	SrcByteBody    []byte `json:"src_byte_body"`
	SrcByteResBody []byte `json:"src_byte_res_body"`
	SrcURL         []byte `json:"src_url"`

	// Truncated marks payload content cut to the configured size limit.
	Truncated int `json:"truncated"`

	// CreateTime matches the request time used to apply payload retention.
	CreateTime  string `gorm:"size:32;index:idx_ep_create_time" json:"create_time"`
	UnixAddTime int64  `json:"unix_add_time"`
	Day         int    `json:"day"`
}

// EventPayloadTableName 实时报文表名。归档分片上是它加分片后缀。
const EventPayloadTableName = "event_payload"

func (EventPayload) TableName() string {
	return EventPayloadTableName
}

// 报文列全空的请求不值得占一行（比如被提前拦下、连 header 都没读到的连接）。
func (e *EventPayload) IsEmpty() bool {
	return e.HEADER == "" && e.COOKIES == "" && e.BODY == "" &&
		e.RES_BODY == "" && e.POST_FORM == "" && e.ResHeader == "" &&
		len(e.SrcByteBody) == 0 && len(e.SrcByteResBody) == 0 && len(e.SrcURL) == 0
}
