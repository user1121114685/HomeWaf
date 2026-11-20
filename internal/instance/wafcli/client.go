// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package wafcli

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/instance/wafconfig"
	"HomeWaf/internal/platform/utils"
	"HomeWaf/internal/platform/utils/localca"
)

const portsAPI = "/api/v1/pipeline/ports"
const maxResponseBytes = 32 << 20

type apiClient struct {
	baseURL string
	key     string
	http    *http.Client
	wait    time.Duration
	host    string
}

func newLocalClient(wait time.Duration) (*apiClient, error) {
	key := strings.TrimSpace(os.Getenv("SAMWAF_API_KEY"))
	if key == "" {
		keyPath := filepath.Join(utils.GetCurrentDir(), "data", ".keys", "cli_api_key")
		if err := rejectKeySymlinks(keyPath); err != nil {
			return nil, fail(ExitAuth, errors.New("Key文件路径不正确，请执行 homewaf apikey init"))
		}
		b, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fail(ExitAuth, errors.New("请先执行 homewaf apikey init"))
		}
		key = strings.TrimSpace(string(b))
	}
	if key == "" || strings.ContainsAny(key, "\r\n") {
		return nil, fail(ExitAuth, errors.New("API Key 为空或格式不正确，请执行 homewaf apikey init"))
	}
	scheme := "http"
	if global.GWAF_SSL_ENABLE && global.GWAF_SSL_FORCE_HTTPS {
		scheme = "https"
	}
	base := fmt.Sprintf("%s://127.0.0.1:%d", scheme, global.GWAF_LOCAL_SERVER_PORT)
	if global.GWAF_SECURITY_ENTRY_ENABLE {
		entry := strings.Trim(global.GWAF_SECURITY_ENTRY_PATH, "/")
		if entry != "" {
			base += "/" + url.PathEscape(entry)
		}
	}
	client := newAPIClient(base, key, wait)
	domain := wafconfig.GetManagementDomain()
	if scheme == "https" {
		config, err := localManagementTLS(localca.DefaultPaths(utils.GetCurrentDir()).SrvCert, domain)
		if err != nil {
			return nil, fail(ExitAuth, err)
		}
		client.http.Transport.(*http.Transport).TLSClientConfig = config
	}
	if domain != "" {
		client.host = net.JoinHostPort(domain, fmt.Sprint(global.GWAF_LOCAL_SERVER_PORT))
	}
	return client, nil
}

// The local client already has access to the certificate installed by the
// management server. Trust that exact leaf even for a self-signed installation.
func localManagementTLS(certPath, domain string) (*tls.Config, error) {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("读取管理端证书失败: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("管理端证书格式无效")
	}
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	roots := x509.NewCertPool()
	roots.AddCert(leaf)
	name := domain
	if name == "" {
		name = "127.0.0.1"
		if leaf.VerifyHostname(name) != nil {
			if len(leaf.DNSNames) > 0 {
				name = leaf.DNSNames[0]
			} else if len(leaf.IPAddresses) > 0 {
				name = leaf.IPAddresses[0].String()
			}
		}
	}
	return &tls.Config{RootCAs: roots, ServerName: name, VerifyConnection: func(state tls.ConnectionState) error {
		if len(state.PeerCertificates) == 0 || !state.PeerCertificates[0].Equal(leaf) {
			return errors.New("管理端证书与本机配置不匹配")
		}
		return nil
	}}, nil
}

func newAPIClient(base, key string, wait time.Duration) *apiClient {
	tr := &http.Transport{
		Proxy:       nil,
		DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
	}
	return &apiClient{baseURL: strings.TrimRight(base, "/"), key: key, wait: wait,
		http: &http.Client{Transport: tr, Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (a *apiClient) call(ctx context.Context, method, path string, body any, out any) error {
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return fail(ExitUsage, errors.New("无法编码请求JSON"))
		}
	}
	deadline := time.Now().Add(a.wait)
	for {
		requestCtx := ctx
		cancel := func() {}
		if a.wait > 0 {
			requestCtx, cancel = context.WithDeadline(ctx, deadline)
		}
		req, e := http.NewRequestWithContext(requestCtx, method, a.baseURL+path, bytes.NewReader(payload))
		if e != nil {
			cancel()
			return fail(ExitUsage, errors.New("管理端地址格式错误"))
		}
		req.Header.Set("X-API-Key", a.key)
		if a.host != "" {
			req.Host = a.host
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, e := a.http.Do(req)
		if e != nil {
			cancel()
			// 只重试尚未建立连接的请求，避免重复提交写操作。
			var op *net.OpError
			retryable := errors.As(e, &op) && op.Op == "dial"
			if retryable && a.wait > 0 && time.Now().Before(deadline) {
				delay := time.Until(deadline)
				if delay > time.Second {
					delay = time.Second
				}
				select {
				case <-ctx.Done():
					return fail(ExitUnavailable, errors.New("操作已取消"))
				case <-time.After(delay):
				}
				continue
			}
			return fail(ExitUnavailable, fmt.Errorf("无法连接 HomeWaf 管理端，请检查服务、端口与HTTPS配置；可使用 --wait 60s 等待启动"))
		}
		b, e := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
		resp.Body.Close()
		cancel()
		if e != nil || len(b) > maxResponseBytes {
			return fail(ExitFailed, errors.New("读取管理端响应失败或响应过大"))
		}
		if resp.StatusCode != http.StatusOK {
			hint := ""
			if resp.StatusCode == 403 {
				hint = "，请检查管理端IP白名单、域名白名单与安全入口"
			}
			return fail(ExitFailed, fmt.Errorf("管理端返回HTTP %d%s", resp.StatusCode, hint))
		}
		var envelope struct {
			Code *int            `json:"code"`
			Data json.RawMessage `json:"data"`
			Msg  string          `json:"msg"`
		}
		if json.Unmarshal(b, &envelope) != nil || envelope.Code == nil {
			return fail(ExitFailed, errors.New("管理端响应不是有效的API结果，请检查安全入口和HTTPS配置"))
		}
		if *envelope.Code != 0 {
			code := ExitFailed
			if *envelope.Code == -999 {
				code = ExitAuth
			}
			msg := envelope.Msg
			if msg == "" {
				msg = "管理端拒绝操作"
			}
			msg = strings.ReplaceAll(msg, a.key, "[REDACTED]")
			return fail(code, errors.New(msg))
		}
		if out != nil && json.Unmarshal(envelope.Data, out) != nil {
			return fail(ExitFailed, errors.New("管理端返回的数据格式不正确"))
		}
		return nil
	}
}
