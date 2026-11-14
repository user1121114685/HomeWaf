package utils

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"testing"
)

func TestOutboundRequestErrorPreservesCauseWithoutSecrets(t *testing.T) {
	endpoint := "https://account:private-password@api.example.com/device-secret?access_key=query-secret"
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"dns", &net.DNSError{Err: "raw-secret", Name: "api.example.com", IsNotFound: true}, "DNS 解析失败"},
		{"timeout", context.DeadlineExceeded, "超时"},
		{"tls", &tls.CertificateVerificationError{Err: errors.New("raw-secret")}, "TLS 证书校验失败"},
		{"proxy", &net.OpError{Op: "proxyconnect", Net: "tcp", Err: errors.New("raw-secret")}, "代理连接失败"},
		{"policy", fmt.Errorf("%w: private-target", ErrOutboundAddressDenied), "security.outbound_allowed_hosts"},
		{"unknown", errors.New("raw-secret"), "网络连接失败"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := OutboundRequestError("通知请求", endpoint, &url.Error{Op: "Post", URL: endpoint, Err: tc.err})
			message := err.Error()
			if !strings.Contains(message, tc.want) || !strings.Contains(message, "api.example.com") {
				t.Fatalf("cause lost: %s", message)
			}
			for _, secret := range []string{"private-password", "device-secret", "query-secret", "raw-secret", "private-target"} {
				if strings.Contains(message, secret) {
					t.Fatal("error exposed credential or raw detail")
				}
			}
			if tc.name == "dns" && strings.Contains(message, "security.outbound_allowed_hosts") {
				t.Fatal("DNS failure incorrectly recommends an allowlist exception")
			}
		})
	}
}
