package wafupdate

import (
	"HomeWaf/global"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/mod/semver"
)

func normalizedVersion(version string) string {
	return "v" + strings.TrimPrefix(version, "v")
}

func (u *Updater) fetchInfoGithub() error {
	return u.fetchReleaseInfo(global.GUPDATE_GITHUB_VERSION_URL, true)
}

// A release endpoint is separate from the legacy manifest/announcement/data root.
func (u *Updater) fetchReleaseInfo(endpoint string, allowPrerelease bool) error {
	u.BinGithubURL, u.ArchiveSHA256 = "", ""
	if endpoint == "" {
		return errors.New("所选 HomeWaf 更新渠道已禁用，请配置对应更新源")
	}
	r, err := u.fetch(endpoint)
	if err != nil {
		var status *HTTPStatusError
		if errors.As(err, &status) {
			switch status.StatusCode {
			case http.StatusNotFound:
				return ErrNoPublishedRelease
			case http.StatusForbidden, http.StatusTooManyRequests:
				return fmt.Errorf("更新源拒绝请求或请求已限流（HTTP %d），请稍后重试", status.StatusCode)
			}
		}
		return err
	}
	defer r.Close()
	var release struct {
		TagName    string `json:"tag_name"`
		Name       string `json:"name"`
		Body       string `json:"body"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
		Assets     []struct {
			Name   string `json:"name"`
			URL    string `json:"browser_download_url"`
			Digest string `json:"digest"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(r).Decode(&release); err != nil {
		return fmt.Errorf("无法解析 HomeWaf release 元数据: %w", err)
	}
	version := normalizedVersion(release.TagName)
	if !semver.IsValid(version) {
		return errors.New("release 版本号无效，须使用 v主版本.次版本.修订号")
	}
	if release.Draft || strings.Contains(strings.ToLower(release.TagName+" "+release.Name), "debug") || (!allowPrerelease && (release.Prerelease || semver.Prerelease(version) != "")) {
		return ErrNoPublishedRelease
	}
	platform := map[string]string{
		"windows-amd64": "Windows_x86_64", "linux-amd64": "Linux_x86_64", "linux-arm64": "Linux_arm64",
		"darwin-amd64": "Darwin_x86_64", "darwin-arm64": "Darwin_arm64",
	}[plat]
	if platform == "" {
		return fmt.Errorf("HomeWaf release 不支持当前平台 %s", plat)
	}
	extension := ".tar.gz"
	if strings.HasPrefix(plat, "windows-") {
		extension = ".zip"
	}
	expectedName := "HomeWaf_" + platform + "_" + strings.TrimPrefix(version, "v") + extension
	for _, asset := range release.Assets {
		if asset.Name != expectedName {
			continue
		}
		parsed, err := url.Parse(asset.URL)
		if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || !strings.HasSuffix(parsed.Path, extension) {
			return errors.New("invalid HTTPS release asset URL")
		}
		digest, err := hex.DecodeString(strings.TrimPrefix(asset.Digest, "sha256:"))
		if !strings.HasPrefix(asset.Digest, "sha256:") || err != nil || len(digest) != sha256.Size {
			return errors.New("release asset is missing a valid SHA256 digest")
		}
		u.BinGithubURL, u.ArchiveSHA256 = asset.URL, hex.EncodeToString(digest)
		u.Info.Version, u.Info.Desc = version, release.Body
		return nil
	}
	return fmt.Errorf("HomeWaf %s 尚未上传当前平台安装包 %s", version, expectedName)
}
