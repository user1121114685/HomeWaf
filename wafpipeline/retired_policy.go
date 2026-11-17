package wafpipeline

import (
	"bytes"
	"encoding/json"
)

// Retired AI settings can exist in persisted services and exported templates.
// Consume only these known fields on read; never publish or execute them. Keep
// strict decoding for every other field so typos and unsafe options still fail.
func (r *RuleSetConfig) UnmarshalJSON(data []byte) error {
	type current RuleSetConfig
	value := struct {
		*current
		RetiredAI      json.RawMessage `json:"ai"`
		RetiredAIScope json.RawMessage `json:"ai_scope"`
	}{current: (*current)(r)}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(&value)
}
