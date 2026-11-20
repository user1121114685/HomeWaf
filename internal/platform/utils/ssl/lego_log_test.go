package ssl

import (
	"strings"
	"testing"

	"HomeWaf/internal/acmelog"
	legolog "github.com/go-acme/lego/v4/log"
)

func TestSSLOrderLegoLoggerRoutesRedactedEvents(t *testing.T) {
	var messages, levels []string
	acmelog.Capture(func(level, message string) { levels = append(levels, level); messages = append(messages, message) }, func() {
		legolog.Logger.Printf("DNS provider token=%s", "fixture-secret")
		acmeWarn("waiting for DNS propagation")
		legolog.Logger.Fatalf("provider rejected token=%s", "fixture-secret")
	})
	if len(messages) != 3 || levels[2] != "error" {
		t.Fatal("lego log events did not reach the order scope")
	}
	for _, message := range messages {
		if strings.Contains(message, "fixture-secret") {
			t.Fatal("lego logger leaked provider credential")
		}
	}
}
