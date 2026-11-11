package threatip

import (
	"HomeWaf/internal/licensing"
	"HomeWaf/utils"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type SignatureConfig struct{ URL, ManifestURL, FeedID, KeyID, PublicKey string }

var feedIdentifier = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

func ValidateHTTPS(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return errors.New("订阅与签名清单必须使用明确的HTTPS地址，不允许用户凭据或片段")
	}
	if ok, _ := utils.PrecheckOutboundURL(raw); !ok {
		return errors.New("订阅地址不符合出站策略")
	}
	return nil
}
func (c SignatureConfig) Validate() error {
	if err := ValidateHTTPS(c.URL); err != nil {
		return err
	}
	if err := ValidateHTTPS(c.ManifestURL); err != nil {
		return err
	}
	if !feedIdentifier.MatchString(c.FeedID) || !feedIdentifier.MatchString(c.KeyID) {
		return errors.New("签名源标识和密钥ID需为1–64位字母、数字、下划线或连字符")
	}
	raw, _ := json.Marshal(map[string]string{c.KeyID: c.PublicKey})
	if _, err := licensing.ParseKeyring(string(raw)); err != nil {
		return errors.New("请配置可信Ed25519公钥（Base64URL无填充）")
	}
	return nil
}

// FetchSignedSnapshot shares one bounded context for manifest and payload.
// The optional client is used by deterministic local HTTPS integration tests.
// Production passes nil and always uses the application's safe outbound client.
func FetchSignedSnapshot(ctx context.Context, c SignatureConfig, sequence uint64, digest string, client *http.Client) ([]byte, licensing.ThreatSnapshotManifest, FetchStat, error) {
	start := time.Now()
	stat := FetchStat{}
	var manifest licensing.ThreatSnapshotManifest
	if err := c.Validate(); err != nil {
		return nil, manifest, stat, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if client == nil {
		client = utils.SafeOutboundHTTPClient(30 * time.Second)
	}
	copy := *client
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	fetch := func(raw string, limit int64) ([]byte, error) {
		// Syntax/literal check here; the safe transport resolves and validates
		// the exact dial address with this context, avoiding an unbounded extra DNS lookup.
		if ok, _ := utils.PrecheckOutboundURL(raw); !ok {
			return nil, errors.New("订阅地址不符合出站策略")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
		if err != nil {
			return nil, errors.New("无法构造订阅请求")
		}
		req.Header.Set("User-Agent", "HomeWaf-Signed-ThreatSnapshot")
		resp, err := copy.Do(req)
		if err != nil {
			return nil, errors.New("签名订阅连接失败")
		}
		defer resp.Body.Close()
		stat.StatusCode = resp.StatusCode
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("签名订阅返回HTTP %d", resp.StatusCode)
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
		stat.Bytes += len(data)
		if err != nil || int64(len(data)) > limit {
			return nil, errors.New("签名订阅内容超限或不完整")
		}
		return data, nil
	}
	document, err := fetch(c.ManifestURL, 32<<10)
	if err == nil {
		keysJSON, _ := json.Marshal(map[string]string{c.KeyID: c.PublicKey})
		keys, _ := licensing.ParseKeyring(string(keysJSON))
		manifest, err = licensing.VerifyThreatSnapshot(document, c.FeedID, keys, time.Now())
	}
	if err == nil && (manifest.Sequence < sequence || manifest.Sequence == sequence && !strings.EqualFold(manifest.SHA256, digest)) {
		err = errors.New("签名订阅版本回退或同版本内容变化")
	}
	var raw []byte
	if err == nil {
		raw, err = fetch(c.URL, maxFetchBytes)
	}
	if err == nil {
		sum := sha256.Sum256(raw)
		if !strings.EqualFold(hex.EncodeToString(sum[:]), manifest.SHA256) {
			err = errors.New("订阅内容与签名摘要不符")
		}
	}
	stat.Elapsed = time.Since(start)
	if err != nil {
		return nil, manifest, stat, err
	}
	return raw, manifest, stat, nil
}
