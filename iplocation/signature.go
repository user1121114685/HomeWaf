package iplocation

import (
	"HomeWaf/internal/licensing"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
)

func checkSignedDatasetURL(raw string, cfg UpgradeConfig) error {
	if !cfg.Enabled || cfg.UpdateVersionURL == "" {
		return errors.New("未显式启用IP数据库签名源")
	}
	base, err := url.Parse(cfg.UpdateVersionURL)
	if err != nil || base.Scheme != "https" || base.User != nil {
		return errors.New("IP数据库源必须是无凭据HTTPS地址")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" || !strings.EqualFold(u.Host, base.Host) {
		return errors.New("IP数据库只允许访问用户配置的HTTPS源")
	}
	return nil
}
func datasetSigningMessage(m *remoteManifest) []byte {
	payload := struct {
		Version   string                `json:"version"`
		Changelog string                `json:"changelog"`
		Files     map[string]remoteFile `json:"files"`
	}{m.Version, m.Changelog, m.Files}
	data, _ := json.Marshal(payload)
	return append([]byte("HomeWaf ip dataset v1\n"+m.KeyID+"\n"), data...)
}
func verifyDatasetManifest(m *remoteManifest, cfg UpgradeConfig) error {
	keys, err := licensing.ParseKeyring(cfg.TrustedKeysJSON)
	if err != nil || len(keys) == 0 {
		return errors.New("IP数据库可信签名公钥库为空或无效")
	}
	sig, err := base64.RawURLEncoding.DecodeString(m.Signature)
	key := keys[m.KeyID]
	if err != nil || len(key) != ed25519.PublicKeySize || !ed25519.Verify(key, datasetSigningMessage(m), sig) {
		return errors.New("IP数据库清单签名无效")
	}
	if m.Version == "" || len(m.Files) > 16 {
		return errors.New("IP数据库清单元数据无效")
	}
	for _, file := range m.Files {
		digest, e := hex.DecodeString(file.SHA256)
		if e != nil || len(digest) != 32 || file.Size < 1 || file.Size > maxDatasetBytes {
			return errors.New("签名清单必须提供有效SHA256和受限文件大小")
		}
		if err := checkSignedDatasetURL(file.URL, cfg); err != nil {
			return err
		}
	}
	return nil
}
