package waf_service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/model/baseorm"
	"HomeWaf/internal/model/customtype"
	"HomeWaf/internal/platform/common/uuid"
	"HomeWaf/internal/platform/utils"
	"gorm.io/gorm"
)

type ZeroSSLAccountStatus struct {
	HasAccessKey  bool `json:"has_access_key"`
	HasEABKID     bool `json:"has_eab_kid"`
	HasEABHMACKey bool `json:"has_eab_hmac_key"`
	Configured    bool `json:"configured"`
}

func GetZeroSSLAccountStatus() ZeroSSLAccountStatus {
	settings := global.CurrentSettings()
	status := ZeroSSLAccountStatus{HasAccessKey: settings.GCONFIG_ZEROSSL_ACCESS_KEY != "", HasEABKID: settings.GCONFIG_ZEROSSL_EAB_KID != "", HasEABHMACKey: settings.GCONFIG_ZEROSSL_EAB_HMAC_KEY != ""}
	status.Configured = status.HasAccessKey || status.HasEABKID && status.HasEABHMACKey
	return status
}

type ZeroSSLAccountInput struct {
	Mode       string `json:"mode"`
	AccessKey  string `json:"access_key"`
	EABKID     string `json:"eab_kid"`
	EABHMACKey string `json:"eab_hmac_key"`
}

var zeroSSLAccountMu sync.Mutex
var zeroSSLFetchMu sync.Mutex
var errZeroSSLAccountChanged = errors.New("ZeroSSL 认证配置已变更，请重试申请")

const ZeroSSLAccountMissing = "ZeroSSL 尚未配置账户认证，请点击申请平台下方的“配置 ZeroSSL 认证”，填写 API Access Key 或完整的 EAB 凭据"

// EnsureZeroSSLAccount is shared by create, manual renewal and background renewal.
func EnsureZeroSSLAccount() error {
	zeroSSLFetchMu.Lock()
	defer zeroSSLFetchMu.Unlock()
	settings := global.CurrentSettings()
	if settings.GCONFIG_ZEROSSL_EAB_KID != "" && settings.GCONFIG_ZEROSSL_EAB_HMAC_KEY != "" {
		return nil
	}
	if settings.GCONFIG_ZEROSSL_ACCESS_KEY == "" {
		return errors.New(ZeroSSLAccountMissing)
	}
	client := newZeroSSLHTTPClient()
	defer client.CloseIdleConnections()
	kid, hmacKey, err := requestZeroSSLEAB(client, settings.GCONFIG_ZEROSSL_ACCESS_KEY)
	if err != nil {
		return err
	}
	return SaveZeroSSLFetchedEAB(settings.GCONFIG_ZEROSSL_ACCESS_KEY, kid, hmacKey)
}

func newZeroSSLHTTPClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	}}
}

func requestZeroSSLEAB(client *http.Client, accessKey string) (string, string, error) {
	endpoint := "https://api.zerossl.com/acme/eab-credentials?" + url.Values{"access_key": {accessKey}}.Encode()
	req, err := http.NewRequest(http.MethodPost, endpoint, nil)
	if err != nil {
		return "", "", errors.New("无法创建 ZeroSSL 认证请求")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	// HTTP errors may contain the complete URL (and Access Key); never return/log them.
	if err != nil {
		return "", "", utils.OutboundRequestError("ZeroSSL 认证接口请求", endpoint, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("ZeroSSL 认证接口返回 HTTP %d，请检查账户和 Access Key", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil || len(body) > 1<<20 {
		return "", "", errors.New("无法读取 ZeroSSL 认证响应")
	}
	var result struct {
		Success json.RawMessage `json:"success"`
		KID     string          `json:"eab_kid"`
		HMAC    string          `json:"eab_hmac_key"`
		Error   *struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", errors.New("ZeroSSL 认证响应格式无效")
	}
	if string(result.Success) != "true" && string(result.Success) != "1" {
		if result.Error != nil {
			return "", "", fmt.Errorf("ZeroSSL 拒绝获取 EAB（错误码 %d），请检查 Access Key 或直接配置 EAB", result.Error.Code)
		}
		return "", "", errors.New("ZeroSSL 拒绝获取 EAB，请检查 Access Key 或直接配置 EAB")
	}
	if !validZeroSSLSecret(result.KID) || !validZeroSSLSecret(result.HMAC) {
		return "", "", errors.New("ZeroSSL 返回的 EAB 凭据无效")
	}
	return result.KID, result.HMAC, nil
}

func validZeroSSLSecret(value string) bool {
	return value != "" && value != "****" && value != ConfigClearSentinel && len(value) <= 8192 && !strings.ContainsAny(value, "\x00\r\n\t ")
}

// SaveZeroSSLAccount stores credentials together; no intermediate half-EAB state.
func SaveZeroSSLAccount(input ZeroSSLAccountInput) error {
	input.AccessKey, input.EABKID, input.EABHMACKey = strings.TrimSpace(input.AccessKey), strings.TrimSpace(input.EABKID), strings.TrimSpace(input.EABHMACKey)
	values := map[string]string{}
	switch input.Mode {
	case "access_key":
		if !validZeroSSLSecret(input.AccessKey) {
			return errors.New("请填写有效的 ZeroSSL API Access Key")
		}
		// A different API account must not reuse the previous account's cached EAB.
		values["zerossl_access_key"] = input.AccessKey
		values["zerossl_eab_kid"], values["zerossl_eab_hmac_key"] = "", ""
	case "eab":
		if !validZeroSSLSecret(input.EABKID) || !validZeroSSLSecret(input.EABHMACKey) {
			return errors.New("EAB Key ID 和 HMAC Key 必须同时填写")
		}
		values["zerossl_eab_kid"], values["zerossl_eab_hmac_key"] = input.EABKID, input.EABHMACKey
	default:
		return errors.New("请选择 ZeroSSL 认证配置方式")
	}
	zeroSSLAccountMu.Lock()
	defer zeroSSLAccountMu.Unlock()
	return saveZeroSSLValues(values, "")
}

// Avoid applying a network response obtained with an API key that was replaced meanwhile.
func SaveZeroSSLFetchedEAB(accessKey, kid, hmacKey string) error {
	if !validZeroSSLSecret(kid) || !validZeroSSLSecret(hmacKey) {
		return errors.New("ZeroSSL 返回的 EAB 凭据无效")
	}
	zeroSSLAccountMu.Lock()
	defer zeroSSLAccountMu.Unlock()
	settings := global.CurrentSettings()
	if accessKey == "" || settings.GCONFIG_ZEROSSL_ACCESS_KEY != accessKey {
		return errZeroSSLAccountChanged
	}
	if settings.GCONFIG_ZEROSSL_EAB_KID != "" && settings.GCONFIG_ZEROSSL_EAB_HMAC_KEY != "" {
		return nil
	}
	return saveZeroSSLValues(map[string]string{"zerossl_eab_kid": kid, "zerossl_eab_hmac_key": hmacKey}, accessKey)
}

func saveZeroSSLValues(values map[string]string, expectedAccessKey string) error {
	if global.GWAF_LOCAL_DB == nil {
		return errors.New("配置数据库不可用，ZeroSSL 认证未保存")
	}
	encoded := map[string]string{}
	for key, value := range values {
		cipher, err := encConfigValue(key, value)
		if err != nil {
			return err
		}
		encoded[key] = cipher
	}
	err := global.GWAF_LOCAL_DB.Transaction(func(tx *gorm.DB) error {
		if expectedAccessKey != "" {
			var rows []model.SystemConfig
			if err := tx.Where("item IN ?", []string{"zerossl_access_key", "zerossl_eab_kid", "zerossl_eab_hmac_key"}).Find(&rows).Error; err != nil {
				return err
			}
			current := map[string]string{}
			for _, row := range rows {
				current[row.Item] = decConfigValue(row.Item, row.Value)
			}
			if current["zerossl_access_key"] != expectedAccessKey {
				return errZeroSSLAccountChanged
			}
			// Another worker or the manual EAB editor may have completed the pair.
			if current["zerossl_eab_kid"] != "" && current["zerossl_eab_hmac_key"] != "" {
				values = map[string]string{"zerossl_eab_kid": current["zerossl_eab_kid"], "zerossl_eab_hmac_key": current["zerossl_eab_hmac_key"]}
				return nil
			}
		}
		for _, key := range []string{"zerossl_access_key", "zerossl_eab_kid", "zerossl_eab_hmac_key"} {
			value, changed := encoded[key]
			if !changed {
				continue
			}
			var row model.SystemConfig
			err := tx.First(&row, "item = ?", key).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				row = model.SystemConfig{BaseOrm: baseorm.BaseOrm{Id: uuid.GenUUID(), USER_CODE: global.GWAF_USER_CODE, Tenant_ID: global.GWAF_TENANT_ID, CREATE_TIME: customtype.JsonTime(time.Now())}, Item: key, ItemClass: "ssl", ItemType: "string", IsSystem: "1", Remarks: key}
			} else if err != nil {
				return err
			}
			row.Value, row.UPDATE_TIME = value, customtype.JsonTime(time.Now())
			if err := tx.Save(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, errZeroSSLAccountChanged) {
			return errZeroSSLAccountChanged
		}
		return errors.New("ZeroSSL 认证保存失败，请检查配置数据库")
	}
	global.UpdateSettings(func(settings *global.RuntimeSettings) {
		if value, ok := values["zerossl_access_key"]; ok {
			settings.GCONFIG_ZEROSSL_ACCESS_KEY = value
		}
		if value, ok := values["zerossl_eab_kid"]; ok {
			settings.GCONFIG_ZEROSSL_EAB_KID = value
		}
		if value, ok := values["zerossl_eab_hmac_key"]; ok {
			settings.GCONFIG_ZEROSSL_EAB_HMAC_KEY = value
		}
	})
	return nil
}
