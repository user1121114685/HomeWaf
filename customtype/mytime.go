package customtype

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

type JsonTime time.Time

func (t JsonTime) MarshalJSON() ([]byte, error) {
	tm := time.Time(t)
	if !tm.IsZero() {
		tm = tm.In(time.Local)
	}
	return json.Marshal(tm.Format("2006-01-02 15:04:05"))
}

func (t *JsonTime) UnmarshalJSON(data []byte) error {
	var value *string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if value == nil {
		*t = JsonTime(time.Time{})
		return nil
	}
	return t.parse(*value)
}

func (t *JsonTime) parse(value string) error {
	if value == "0001-01-01 00:00:00" || value == "0000-00-00 00:00:00" {
		*t = JsonTime(time.Time{})
		return nil
	}
	if tm, err := time.Parse(time.RFC3339Nano, value); err == nil {
		*t = JsonTime(tm)
		return nil
	}
	tm, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.Local)
	if err != nil {
		return err
	}
	*t = JsonTime(tm)
	return nil
}

func (t JsonTime) Value() (driver.Value, error) {
	tm := time.Time(t)
	if tm.IsZero() {
		return nil, nil // MySQL strict mode rejects 0000-00-00; use NULL for unset times
	}
	return tm, nil
}

func (t *JsonTime) Scan(value interface{}) error {
	if value == nil {
		*t = JsonTime(time.Time{})
		return nil
	}
	switch st := value.(type) {
	case time.Time:
		*t = JsonTime(st)
	case string:
		return t.parse(st)
	case []byte:
		return t.parse(string(st))
	default:
		return fmt.Errorf("unsupported JsonTime scan type %T", value)
	}
	return nil
}
func (t *JsonTime) ToPtr() *JsonTime {
	return t
}
