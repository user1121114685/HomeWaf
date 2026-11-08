// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package driver

import (
	"testing"
	"time"
)

func TestLegacyCredentialsAndTimeCompatibility(t *testing.T) {
	config, err := ParseConfig("operator:p@ss/with:#percent%@tcp(127.0.0.1:3306)/homewaf?parseTime=True&loc=Local&charset=utf8mb4")
	if err != nil {
		t.Fatal(err)
	}
	if config.Connector.Password != "p@ss/with:#percent%" || config.Connector.DB != "homewaf" || config.Loc != time.Local {
		t.Fatalf("legacy DSN changed: %#v", config)
	}
	location := time.FixedZone("fixture", 8*3600)
	for _, value := range []string{"2026-11-09", "2026-11-09 12:30:45", "2026-11-09 12:30:45.123456"} {
		parsed, err := parseDate(value, location)
		if err != nil || parsed.Location() != location {
			t.Fatalf("datetime %q: %v", value, err)
		}
	}
	zero, err := parseDate("0000-00-00 00:00:00", location)
	if err != nil || !zero.IsZero() {
		t.Fatal("zero date compatibility")
	}
}
