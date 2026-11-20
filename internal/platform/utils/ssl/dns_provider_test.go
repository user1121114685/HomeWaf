package ssl

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"HomeWaf/internal/dnsproviders"
	"HomeWaf/internal/model"
	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/providers/dns/hurricane"
)

type dnsRoundTripper func(*http.Request) (*http.Response, error)

func (f dnsRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestHurricaneDDNSAndSequentialWrapper(t *testing.T) {
	t.Setenv("LEGO_DISABLE_CNAME_SUPPORT", "true")
	var requests []url.Values
	config := hurricane.NewDefaultConfig()
	config.Credentials = map[string]string{"example.com": "test-ddns-token"}
	config.SequenceInterval = 3 * time.Second
	config.HTTPClient = &http.Client{Transport: dnsRoundTripper(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://dyn.dns.he.net/nic/update" || r.Method != http.MethodPost {
			t.Fatalf("wrong DDNS endpoint: %s %s", r.Method, r.URL)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		requests = append(requests, r.PostForm)
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("good")), Request: r}, nil
	})}
	inner, err := hurricane.NewDNSProviderConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	provider := newLoggingDNSProvider(inner, "hurricane")
	s, ok := provider.(interface{ Sequential() time.Duration })
	if !ok || s.Sequential() != 3*time.Second {
		t.Fatal("sequential mode lost; apex and wildcard TXT would overwrite each other")
	}
	if _, ok := provider.(challenge.ProviderTimeout); !ok {
		t.Fatal("provider timeout lost")
	}
	for _, domain := range []string{"example.com", "*.example.com"} {
		// ACME wildcard authorizations carry the base domain in Identifier.Value.
		identifier := strings.TrimPrefix(domain, "*.")
		if err := provider.Present(identifier, "challenge-token", "authorization-"+domain); err != nil {
			t.Fatal(err)
		}
		if err := provider.CleanUp(identifier, "challenge-token", "authorization-"+domain); err != nil {
			t.Fatal(err)
		}
	}
	if len(requests) != 4 {
		t.Fatalf("got %d requests", len(requests))
	}
	for i, request := range requests {
		if request.Get("hostname") != "_acme-challenge.example.com" || request.Get("password") != "test-ddns-token" {
			t.Fatal("DDNS record/token mapping incorrect")
		}
		if i%2 == 1 && request.Get("txt") != "." {
			t.Fatal("cleanup must preserve pre-created record")
		}
		if i%2 == 0 && (request.Get("txt") == "" || request.Get("txt") == ".") {
			t.Fatal("challenge TXT missing")
		}
	}
}

func TestDNSRealFactoryAndEarlyValidation(t *testing.T) {
	for _, code := range []string{"cloudflare", "hurricane", "duckdns", "porkbun"} {
		t.Run(code, func(t *testing.T) {
			values := map[string]string{}
			if code == "hurricane" {
				values["HURRICANE_TOKENS"] = "example.com:test-token"
			}
			err := dnsproviders.WithEnvironment(code, values, func() error {
				_, err := GetDnsProvider(code)
				if err != nil && strings.Contains(err.Error(), "unrecognized DNS provider") {
					t.Fatal(err)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, renew := range []bool{false, true} {
		order := model.SslOrder{ApplyMethod: "dns01", ApplyDns: "manual"}
		var err error
		if renew {
			_, err = ReNewSSL(order, "", "http://must-not-connect.invalid", "", "", "")
		} else {
			_, err = RegistrationSSL(order, "", "http://must-not-connect.invalid", "", "", "")
		}
		if err == nil || !strings.Contains(err.Error(), "终端") {
			t.Fatalf("must validate provider before contacting CA: %v", err)
		}
	}
}
