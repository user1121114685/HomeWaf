package acmelog

import (
	"strings"
	"sync"
	"testing"

	"HomeWaf/internal/dnsproviders"
)

func TestACMELogsRedactCredentialsAndPrivateKeys(t *testing.T) {
	raw := "request https://user:password-value@dns.example/update?token=query-secret token=literal-secret -----BEGIN EC PRIVATE KEY-----\nprivate-secret\n-----END EC PRIVATE KEY-----"
	clean := Sanitize(raw)
	for _, secret := range []string{"password-value", "query-secret", "literal-secret", "private-secret"} {
		if strings.Contains(clean, secret) {
			t.Fatalf("leaked %s", secret)
		}
	}
	if !strings.Contains(clean, "dns.example/update") {
		t.Fatal("redaction removed actionable endpoint")
	}
	if strings.Contains(Sanitize(`{"api_token":"json-secret","Authorization":"Bearer auth-secret"}`), "json-secret") || strings.Contains(Sanitize(`{"Authorization":"Bearer auth-secret"}`), "auth-secret") {
		t.Fatal("JSON credential field leaked")
	}
	err := dnsproviders.WithEnvironment("hurricane", map[string]string{"HURRICANE_TOKENS": "example.com:fixture-ddns-secret"}, func() error {
		if strings.Contains(Sanitize("provider echoed fixture-ddns-secret"), "fixture-ddns-secret") {
			t.Fatal("bare provider credential not redacted")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len([]rune(Sanitize(strings.Repeat("测", 3000)))) > 2001 {
		t.Fatal("unbounded log entry")
	}
}

func TestACMELogScopesRemainIsolatedAndRecoverAfterPanic(t *testing.T) {
	var wg sync.WaitGroup
	for _, name := range []string{"first-order", "second-order"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			Capture(func(_ string, msg string) {
				if msg != name {
					t.Errorf("cross-order log: %s", msg)
				}
			}, func() { Emit("info", name) })
		}()
	}
	wg.Wait()
	called := false
	func() {
		defer func() { _ = recover() }()
		Capture(func(string, string) { called = true }, func() { panic("fixture") })
	}()
	Emit("info", "outside scope")
	if called {
		t.Fatal("panic leaked active log scope")
	}
}
