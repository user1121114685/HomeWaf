package innerbean_test

import (
	"HomeWaf/common/queue"
	"HomeWaf/innerbean"
	"encoding/json"
	"strings"
	"testing"
)

func TestEveryQueuedLogIsAnIndependentRedactedCopy(t *testing.T) {
	log := &innerbean.WebLog{REQ_UUID: "r1", URL: "/login?api_key=query-secret", RawQuery: "password=query-password",
		HEADER:    "Authorization: Basic header-secret\r\nX-HomeWaf-Token: token-secret\r\nCookie: c=cookie-secret\r\nUser-Agent: ordinary-client\r\n",
		COOKIES:   `[{"Name":"innocent-name","Value":"cookie-secret","Raw":"raw-secret"}]`,
		BODY:      `{"nested":{"password":"body-password","access_token":"body-token"},"note":"ordinary-content"}`,
		POST_FORM: "username=alice&pa%73sword=form-secret", RES_BODY: `{"token":"response-token","ok":true}`,
		ResHeader:   "Set-Cookie: id=response-cookie\r\nLocation: /next?token=redirect-token\r\nX-Auth: response-header-token\r\n",
		SrcByteBody: []byte("original-bytes-secret"), SrcByteResBody: []byte("original-response-secret"),
		REFERER: "https://example.test/?token=referer-token", BodyDecoded: "decoded-secret", BodyValues: []string{"decoded-secret"}}
	q := queue.NewQueue()
	q.Enqueue(log)
	v, ok := q.Dequeue()
	if !ok {
		t.Fatal("missing log")
	}
	safe := v.(*innerbean.WebLog)
	data, err := json.Marshal(safe)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"query-secret", "query-password", "header-secret", "token-secret", "cookie-secret", "raw-secret", "body-password", "body-token", "form-secret", "response-token", "response-cookie", "redirect-token", "response-header-token", "original-bytes-secret", "original-response-secret", "referer-token", "decoded-secret"} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("credential leaked: %s", secret)
		}
	}
	if !strings.Contains(safe.BODY, "ordinary-content") || !strings.Contains(safe.HEADER, "ordinary-client") {
		t.Fatal("lost structured diagnostic context")
	}
	if !strings.Contains(log.BODY, "body-password") || len(log.SrcByteBody) == 0 {
		t.Fatal("modified detection source")
	}
	log.BODY = "later mutation"
	if safe.BODY == log.BODY || safe.BodyDecoded != "" || safe.BodyValues != nil {
		t.Fatal("retained mutable request data")
	}
	for _, body := range []string{`{"password":"unterminated`, "--multipart-secret", strings.Repeat("x", 65537)} {
		safe := (&innerbean.WebLog{BODY: body}).RedactedCopy()
		if safe.BODY != "[MASKED]" {
			t.Fatal("unsafe unstructured body retained")
		}
	}
}
