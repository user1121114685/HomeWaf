package dnsproviders

import (
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestDNSCredentialScopeRestoresAndSerializes(t *testing.T) {
	t.Setenv("CF_DNS_API_TOKEN", "ambient-secret")
	t.Setenv("CF_DNS_API_TOKEN_FILE", "ambient-file")
	t.Setenv("HURRICANE_TOKENS", "ambient-hurricane")
	var wg sync.WaitGroup
	for _, secret := range []string{"first-token", "second-token"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := WithEnvironment("cloudflare", map[string]string{"CF_DNS_API_TOKEN": secret}, func() error {
				if os.Getenv("CF_DNS_API_TOKEN") != secret {
					t.Error("credentials crossed between orders")
				}
				if os.Getenv("CF_DNS_API_TOKEN_FILE") != "" || os.Getenv("HURRICANE_TOKENS") != "" {
					t.Error("ambient credentials leaked into order")
				}
				return errors.New("SDK failure token=" + secret)
			})
			if err == nil || strings.Contains(err.Error(), secret) {
				t.Error("credential was returned in error")
			}
		}()
	}
	wg.Wait()
	if os.Getenv("CF_DNS_API_TOKEN") != "ambient-secret" || os.Getenv("CF_DNS_API_TOKEN_FILE") != "ambient-file" || os.Getenv("HURRICANE_TOKENS") != "ambient-hurricane" {
		t.Fatal("environment not restored")
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("test panic missing")
			}
		}()
		_ = WithEnvironment("cloudflare", map[string]string{"CF_DNS_API_TOKEN": "panic-token"}, func() error { panic("stop") })
	}()
	if os.Getenv("CF_DNS_API_TOKEN") != "ambient-secret" {
		t.Fatal("panic leaked credential")
	}
}

func TestDNSRejectsUnrelatedEnvironmentAndManual(t *testing.T) {
	for _, tc := range []struct{ provider, key, value string }{
		{"cloudflare", "PATH", "untrusted"}, {"cloudflare", "CF_DNS_API_TOKEN_FILE", "untrusted"},
		{"cloudflare", "CF_DNS_API_TOKEN", "****"}, {"manual", "", ""}, {"unknown", "", ""},
	} {
		called := false
		err := WithEnvironment(tc.provider, map[string]string{tc.key: tc.value}, func() error { called = true; return nil })
		if err == nil || called {
			t.Fatalf("invalid credential scope accepted: %s/%s", tc.provider, tc.key)
		}
	}
}

func TestHurricaneTokens(t *testing.T) {
	values, err := ParseHurricaneTokens("example.com:token123,nas.example.com:token456")
	if err != nil || len(values) != 2 || values["example.com"] != "token123" {
		t.Fatalf("valid tokens rejected: %v", err)
	}
	for _, bad := range []string{"", "token123", "*.example.com:secret", "_acme-challenge.example.com:secret", "example.com:", "example.com:secret,example.com:other", "example.com:line\nbreak"} {
		if _, err := ParseHurricaneTokens(bad); err == nil {
			t.Errorf("invalid tokens accepted: %q", bad)
		}
	}
	err = WithEnvironment("hurricane", map[string]string{"HURRICANE_TOKENS": "example.com:privateDDNSkey"}, func() error { return errors.New("failed privateDDNSkey") })
	if err == nil || strings.Contains(err.Error(), "privateDDNSkey") {
		t.Fatal("DDNS key leaked")
	}
}

func TestDNSCatalogue(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range List().Providers {
		if p.Code == "" || seen[p.Code] || p.Name == "" {
			t.Fatalf("invalid catalogue provider %q", p.Code)
		}
		seen[p.Code] = true
		for _, alias := range p.Aliases {
			resolved, ok := Lookup(alias)
			if !ok || resolved.Code != p.Code {
				t.Fatalf("lost alias %s", alias)
			}
		}
		if p.Code != "manual" && len(p.Fields) == 0 {
			t.Fatalf("missing fields for %s", p.Code)
		}
	}
	if len(seen) < 190 {
		t.Fatalf("incomplete catalogue: %d", len(seen))
	}
}
