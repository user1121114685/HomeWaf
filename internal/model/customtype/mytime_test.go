package customtype

import (
	"encoding/json"
	"testing"
	"time"
)

func TestJsonTimeRoundTripKeepsInstantAcrossLocations(t *testing.T) {
	old := time.Local
	time.Local = time.FixedZone("fixture-east", 8*60*60)
	t.Cleanup(func() { time.Local = old })
	instant := time.Date(2026, 11, 11, 3, 4, 5, 0, time.UTC)
	for _, input := range []time.Time{instant, instant.In(time.Local), instant.In(time.FixedZone("fixture-west", -5*60*60))} {
		raw, err := json.Marshal(JsonTime(input))
		if err != nil {
			t.Fatal(err)
		}
		if string(raw) != `"2026-11-11 11:04:05"` {
			t.Fatalf("local representation=%s", raw)
		}
		var output JsonTime
		if err = json.Unmarshal(raw, &output); err != nil {
			t.Fatal(err)
		}
		if !time.Time(output).Equal(input) {
			t.Fatalf("cache shifted instant: %s -> %s", input, time.Time(output))
		}
	}
	var output JsonTime
	if err := json.Unmarshal([]byte(`"2026-11-11T03:04:05Z"`), &output); err != nil || !time.Time(output).Equal(instant) {
		t.Fatal("explicit timezone lost", err)
	}
	for _, input := range []any{"2026-11-11 11:04:05", []byte("2026-11-11 11:04:05"), instant} {
		if err := output.Scan(input); err != nil || !time.Time(output).Equal(instant) {
			t.Fatal("SQL scan changed instant", input, err)
		}
	}
	if err := output.Scan(nil); err != nil || !time.Time(output).IsZero() {
		t.Fatal("NULL did not clear old date")
	}
	for _, input := range []string{`null`, `"0001-01-01 00:00:00"`} {
		if err := json.Unmarshal([]byte(input), &output); err != nil || !time.Time(output).IsZero() {
			t.Fatal("zero value changed", err)
		}
	}
	if err := json.Unmarshal([]byte(`"not-a-date"`), &output); err == nil {
		t.Fatal("invalid date accepted")
	}
}
