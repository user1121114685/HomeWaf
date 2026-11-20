package wafcli

import (
	"strings"
	"testing"
)

func TestReadSiteInputRejectsInvalidBatchBeforeWrites(t *testing.T) {
	for _, input := range []string{"{}", "[]", `{"port":1,"remote_host":"http://old"}`, `[{"port":1},{"port":1}]`, `{"port":65536}`, `{"port":1}{}`} {
		if _, err := readSiteInput("-", strings.NewReader(input)); errorCode(err) != ExitUsage {
			t.Fatalf("invalid input accepted: %q: %v", input, err)
		}
	}
	if sites, err := readSiteInput("-", strings.NewReader(`[{"port":8080},{"port":8081}]`)); err != nil || len(sites) != 2 {
		t.Fatal(sites, err)
	}
}
