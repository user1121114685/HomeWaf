package utils

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// OutboundRequestError explains the failure category without including raw URL
// errors, paths, queries or upstream response bodies that can contain credentials.
func OutboundRequestError(operation, endpoint string, err error) error {
	host := "目标服务"
	if parsed, parseErr := url.Parse(endpoint); parseErr == nil && parsed.Hostname() != "" {
		host = parsed.Hostname()
	}
	prefix := fmt.Sprintf("%s失败（%s）", operation, host)
	if errors.Is(err, ErrOutboundAddressDenied) {
		return fmt.Errorf("%s：目标地址被出站安全策略拒绝；确需访问内网服务时，请在 conf/config.yml 的 security.outbound_allowed_hosts 中声明", prefix)
	}
	var dnsError *net.DNSError
	if errors.As(err, &dnsError) {
		return fmt.Errorf("%s：DNS 解析失败，请检查运行 HomeWaf 的机器的 DNS 或代理配置；加入放行清单不能解决 DNS 无法解析", prefix)
	}
	var verification *tls.CertificateVerificationError
	var unknownAuthority x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var invalidCertificate x509.CertificateInvalidError
	if errors.As(err, &verification) || errors.As(err, &unknownAuthority) || errors.As(err, &hostname) || errors.As(err, &invalidCertificate) {
		return fmt.Errorf("%s：TLS 证书校验失败，请检查系统时间、系统信任证书及代理的证书配置", prefix)
	}
	var netError net.Error
	if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &netError) && netError.Timeout() {
		return fmt.Errorf("%s：连接或请求超时，请检查出站网络、防火墙和代理配置", prefix)
	}
	var operationError *net.OpError
	if errors.As(err, &operationError) && strings.Contains(operationError.Op, "proxy") {
		return fmt.Errorf("%s：代理连接失败，请检查 HomeWaf 进程的 HTTPS_PROXY / HTTP_PROXY 配置", prefix)
	}
	return fmt.Errorf("%s：网络连接失败，请检查目标服务、出站网络及代理配置", prefix)
}
