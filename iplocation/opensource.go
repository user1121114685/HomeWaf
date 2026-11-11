package iplocation

import (
	"context"
	"crypto/sha1" // Git blob identity, not an authentication or password primitive.
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/lionsoul2014/ip2region/binding/golang/xdb"
)

const openSourceAPI = "https://api.github.com/repos/lionsoul2014/ip2region"
const openSourceRaw = "https://raw.githubusercontent.com/lionsoul2014/ip2region/"
const maxDatasetBytes int64 = 128 << 20

var gitObjectID = regexp.MustCompile(`^[a-f0-9]{40}$`)
var openSourceCache struct {
	sync.Mutex
	manifest *remoteManifest
	checked  time.Time
}

// UsesOpenSourceUpdates keeps IP data independent from the optional software publisher.
// Explicit publisher mirrors retain their existing manifest protocol.
func UsesOpenSourceUpdates() bool { return currentUpgradeCfg().UpdateVersionURL == "" }

func clearOpenSourceCache() {
	openSourceCache.Lock()
	defer openSourceCache.Unlock()
	openSourceCache.manifest = nil
	openSourceCache.checked = time.Time{}
}

func resolveManifest(ctx context.Context) (*remoteManifest, error) {
	if cfg := currentUpgradeCfg(); cfg.RequireSignature {
		return fetchManifest(ctx, manifestURL(cfg))
	}
	if !UsesOpenSourceUpdates() {
		return fetchManifest(ctx, manifestURL(currentUpgradeCfg()))
	}
	openSourceCache.Lock()
	defer openSourceCache.Unlock()
	if openSourceCache.manifest != nil && time.Since(openSourceCache.checked) < 5*time.Minute {
		return openSourceCache.manifest, nil
	}
	// Only public repository metadata is sent, never installation IDs or account data.
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	var ref struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := fetchOpenSourceJSON(ctx, openSourceAPI+"/git/ref/heads/master", &ref); err != nil {
		return nil, err
	}
	if !gitObjectID.MatchString(ref.Object.SHA) {
		return nil, fmt.Errorf("开源源返回了无效的提交标识")
	}
	var entries []struct {
		Name string `json:"name"`
		SHA  string `json:"sha"`
		Size int64  `json:"size"`
	}
	if err := fetchOpenSourceJSON(ctx, openSourceAPI+"/contents/data?ref="+ref.Object.SHA, &entries); err != nil {
		return nil, err
	}
	m := &remoteManifest{
		Version:   "opensource-" + ref.Object.SHA[:12],
		Changelog: "ip2region 官方开源数据；版本号为文件内容标识，检查结果缓存 5 分钟。",
		Files:     map[string]remoteFile{},
	}
	for _, entry := range entries {
		key := ""
		switch entry.Name {
		case "ip2region_v4.xdb":
			key = "ip2region_v4"
		case "ip2region_v6.xdb":
			key = "ip2region_v6"
		}
		if key == "" {
			continue
		}
		if !gitObjectID.MatchString(entry.SHA) || entry.Size < 256 || entry.Size > maxDatasetBytes {
			return nil, fmt.Errorf("开源数据文件 %s 的标识或大小无效", entry.Name)
		}
		m.Files[key] = remoteFile{
			URL:  openSourceRaw + ref.Object.SHA + "/data/" + entry.Name,
			Size: entry.Size, GitBlobSHA: entry.SHA, Version: "opensource-" + entry.SHA[:12],
		}
	}
	if len(m.Files) == 0 {
		return nil, fmt.Errorf("开源源未提供可下载的 XDB 数据")
	}
	openSourceCache.manifest, openSourceCache.checked = m, time.Now()
	return m, nil
}

func fetchOpenSourceJSON(ctx context.Context, url string, target any) error {
	if err := checkURL(url); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "HomeWaf-IPDB-Updater")
	resp, err := newHTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("无法连接 ip2region 开源更新源: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 403 || resp.StatusCode == 429 {
		return fmt.Errorf("GitHub 开源更新源拒绝请求或已限流（HTTP %d），请稍后重试或手动上传", resp.StatusCode)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("ip2region 开源更新源返回 HTTP %d", resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(target); err != nil {
		return fmt.Errorf("开源更新源返回的数据无法解析: %w", err)
	}
	return nil
}

func verifyGitBlob(path, expected string, expectedSize int64) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	if st.Size() != expectedSize {
		return fmt.Errorf("文件大小校验失败：预期 %d，实际 %d", expectedSize, st.Size())
	}
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", st.Size())
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if hex.EncodeToString(h.Sum(nil)) != expected {
		return fmt.Errorf("文件内容校验失败（Git blob 标识不匹配）")
	}
	return nil
}

func verifyDataset(path, key string) error {
	if err := xdb.VerifyFromFile(path); err != nil {
		return fmt.Errorf("XDB 文件校验失败: %w", err)
	}
	h, err := xdb.LoadHeaderFromFile(path)
	if err != nil {
		return err
	}
	expected := 4
	if key == "ip2region_v6" {
		expected = 6
	}
	if h.IPVersion != expected {
		return fmt.Errorf("XDB 协议不匹配：需要 IPv%d，文件为 IPv%d", expected, h.IPVersion)
	}
	return nil
}
