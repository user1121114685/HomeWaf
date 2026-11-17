package wafmcp

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestRedactionPreservesAttackEvidenceAndConfigurationNames(t *testing.T) {
	value := map[string]any{
		"req_uuid": "request-1", "src_ip": "192.0.2.4", "rule": "sqli", "status_code": 403,
		"header":        "Authorization: Bearer TOP-SECRET\r\nUser-Agent: sqlmap\r\nX-API-Key: PRIVATE-KEY",
		"body":          `{"search":"1 UNION SELECT password FROM users--","password":"LOGIN-SECRET","nested":{"token":"SESSION-SECRET"}}`,
		"src_byte_body": base64.StdEncoding.EncodeToString([]byte("LOGIN-SECRET")),
		"policy":        map[string]any{"password_field": "pwd", "token_header": "X-Service-Token", "credential_group": "dns-prod", "require_token": true},
		"diagnostics":   map[string]any{"state": "available", "body_truncated": true, "hits": []any{map[string]any{"rule_id": "sqli", "code": "libinjection", "observe": false}}},
	}
	wire, _ := json.Marshal(redact(value))
	for _, secret := range []string{"TOP-SECRET", "PRIVATE-KEY", "LOGIN-SECRET", "SESSION-SECRET", base64.StdEncoding.EncodeToString([]byte("LOGIN-SECRET"))} {
		if strings.Contains(string(wire), secret) {
			t.Fatalf("credential exposed: %s", secret)
		}
	}
	for _, evidence := range []string{"UNION SELECT password FROM users", "sqlmap", "request-1", "192.0.2.4", "libinjection", "password_field", "pwd", "X-Service-Token", "dns-prod"} {
		if !strings.Contains(string(wire), evidence) {
			t.Fatalf("necessary evidence/configuration lost: %s", evidence)
		}
	}
}
