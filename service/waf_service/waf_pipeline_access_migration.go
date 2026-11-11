package waf_service

import (
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/utils"
	"HomeWaf/wafenginecore/accessgate"
	"HomeWaf/wafpipeline"
	"errors"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type legacyAccessEvidence struct {
	Enabled           bool
	Config            model.AccessConfig
	Accounts          []model.AccessAccount
	CredentialSecrets []string
	Groups            map[string][]model.IPGroupItem
}

func migrateLegacyAccess(db *gorm.DB, selected model.Hosts, portHosts []model.Hosts, port uint16, c *wafpipeline.PortConfig, out *LegacyPortPreview) (legacyAccessEvidence, error) {
	evidence := legacyAccessEvidence{Config: model.DefaultAccessConfig(), Groups: map[string][]model.IPGroupItem{}}
	addGroup := func(group string) error {
		if group == "" {
			return nil
		}
		var items []model.IPGroupItem
		if err := optionalLegacyRows(db, &items, "group_code = ? AND user_code = ? AND tenant_id = ?", group, selected.USER_CODE, selected.Tenant_ID); err != nil {
			return err
		}
		if len(items) == 0 {
			var groups []model.IPGroup
			if err := optionalLegacyRows(db, &groups, "group_name = ? AND user_code = ? AND tenant_id = ?", group, selected.USER_CODE, selected.Tenant_ID); err != nil {
				return err
			}
			if len(groups) == 1 {
				if err := optionalLegacyRows(db, &items, "group_code = ? AND user_code = ? AND tenant_id = ?", groups[0].GroupCode, selected.USER_CODE, selected.Tenant_ID); err != nil {
					return err
				}
			}
		}
		evidence.Groups[group] = items
		for _, item := range items {
			c.Policy.Gate.ExemptIPs = append(c.Policy.Gate.ExemptIPs, item.Ip)
		}
		return nil
	}
	if selected.IsEnableHttpAuthBase == 1 {
		if err := addGroup(model.DecodeHttpAuthConfig(selected.HttpAuthJSON).BypassIPGroup); err != nil {
			return evidence, err
		}
	}
	var settings []model.SystemConfig
	if err := optionalLegacyRows(db, &settings, "item = ? AND user_code = ? AND tenant_id = ?", "access_enable", selected.USER_CODE, selected.Tenant_ID); err != nil {
		return evidence, err
	}
	enabled := global.CurrentSettings().GCONFIG_ACCESS_ENABLE == 1
	if len(settings) > 0 {
		enabled = strings.TrimSpace(settings[0].Value) == "1"
	}
	if db.Migrator().HasTable(&model.AccessConfig{}) {
		if err := db.Where("user_code = ? AND tenant_id = ?", selected.USER_CODE, selected.Tenant_ID).First(&evidence.Config).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return evidence, err
		}
	}
	evidence.Config.FillDefaults()
	host := model.ParseAccessConfig(selected.AccessJSON)
	evidence.Enabled = !accessgate.Get().ForceDisable && accessgate.IsAccessEnabled(host.Mode, enabled)
	if !evidence.Enabled {
		return evidence, nil
	}
	c.Policy.Gate.Enabled = true
	c.Policy.Gate.Mode = "form"
	c.Policy.Gate.BypassTrusted = false
	c.Policy.Gate.CredentialCookiePrefixes = append(c.Policy.Gate.CredentialCookiePrefixes, evidence.Config.CookiePrefix)
	if evidence.Config.ServiceTokenHeader != "" {
		c.Policy.Gate.CredentialHeaders = append(c.Policy.Gate.CredentialHeaders, evidence.Config.ServiceTokenHeader)
	}
	c.Policy.Gate.Protect = wafpipeline.Scope{}
	c.Policy.Gate.AbsoluteSeconds = evidence.Config.TokenTTLMinutes * 60
	if evidence.Config.IdleTimeoutMinutes > 0 {
		c.Policy.Gate.IdleSeconds = evidence.Config.IdleTimeoutMinutes * 60
	}
	c.Policy.Gate.BindIP = evidence.Config.BindIP == 1
	if evidence.Config.BindFingerprint == 1 {
		out.Warnings = append(out.Warnings, "迁入使用端口独立签名会话和即时撤销，不继承旧SSO的弱设备指纹；可在登录门配置IP绑定")
	}
	c.Policy.Gate.Exempt.Paths = append(c.Policy.Gate.Exempt.Paths, splitLegacyList(evidence.Config.GlobalExcludePaths)...)
	c.Policy.Gate.Exempt.Paths = append(c.Policy.Gate.Exempt.Paths, splitLegacyList(host.ExcludePaths)...)
	if err := addGroup(evidence.Config.BypassIPGroupCode); err != nil {
		return evidence, err
	}
	if err := addGroup(host.AllowIPGroupCode); err != nil {
		return evidence, err
	}
	if evidence.Config.ServiceTokenHeader != "" {
		c.Policy.Gate.TokenHeader = evidence.Config.ServiceTokenHeader
		out.secrets.ServiceTokenHashes = splitLegacyList(evidence.Config.ServiceTokenHashes)
	}
	evidence.CredentialSecrets = append(evidence.CredentialSecrets, evidence.Config.ServiceTokenHashes)
	if err := legacyAccountRows(db, &evidence.Accounts, "status = ? AND user_code = ? AND tenant_id = ?", model.AccessAccountStatusEnable, selected.USER_CODE, selected.Tenant_ID); err != nil {
		return evidence, err
	}
	var users []model.GateUser
	for _, account := range evidence.Accounts {
		evidence.CredentialSecrets = append(evidence.CredentialSecrets, account.Password, account.OtpSecret)
		if !WafAccessAccountServiceApp.IsHostAllowed(&account, selected.Code) {
			continue
		}
		expires := int64(0)
		if date := time.Time(account.ExpireTime); !date.IsZero() {
			expires = date.Unix()
			if expires <= time.Now().Unix() {
				continue
			}
		}
		needsOTP := WafAccessAccountServiceApp.NeedOtp(&account, host.RequireOtp, evidence.Config.RequireOtp == 1)
		out.RequiresTOTP = out.RequiresTOTP || needsOTP
		if validateGateUsername(account.AccountName) != nil {
			out.Warnings = append(out.Warnings, "旧SSO账号名称不合法，需另设账号")
			continue
		}
		if cost, err := bcrypt.Cost([]byte(account.Password)); err != nil || cost != bcrypt.DefaultCost {
			out.Warnings = append(out.Warnings, "旧SSO账号哈希无效或成本与当前登录门不同，需重设账号口令")
			continue
		}
		user := model.GateUser{PortID: wafpipeline.PortID(port), Username: account.AccountName, PasswordHash: account.Password, Enabled: true, ExpiresAt: expires, Remark: account.Remarks}
		if needsOTP {
			out.RequiresTOTP = true
			secret := decryptAccessSecret(account.OtpSecret)
			normalized, err := wafpipeline.NormalizeTOTPSecret(secret)
			if err != nil {
				out.Blockers = append(out.Blockers, "SSO动态口令密钥无法安全迁入，需先修复原账号")
				continue
			}
			user.TOTPSecret = normalized
		}
		users = append(users, user)
	}
	out.accounts = nil
	out.AccountNames = []string{}
	if selected.IsEnableHttpAuthBase == 1 {
		c.Policy.Gate.Exempt = wafpipeline.Scope{}
		c.Policy.Gate.ExemptIPs = nil
		c.Policy.Gate.TokenHeader = ""
		out.secrets.ServiceTokenHashes = nil
		out.Warnings = append(out.Warnings, "原站点同时使用SSO和网站密码；不会把两套账号合并为任选一种放行。请建立新的端口账号；原账号数据保留")
		out.Warnings = append(out.Warnings, "双重旧认证的免登录范围和服务令牌不会合并扩大；新端口先要求完整验证，请在启用前重新核对例外")
	} else {
		out.accounts = users
		for _, user := range users {
			out.AccountNames = append(out.AccountNames, user.Username)
		}
	}
	out.Warnings = append(out.Warnings, "统一SSO迁为端口独立账号与会话，保留账号授权范围、有效期及已要求的动态口令；旧SSO会话不直接复用，后续账号修改彼此独立")
	// Do not dismantle an authentication center still used by old listeners.
	center, err := url.Parse(evidence.Config.CenterOrigin)
	if err == nil && center.Hostname() != "" {
		centerPort := 80
		if center.Scheme == "https" {
			centerPort = 443
		}
		if center.Port() != "" {
			centerPort, _ = strconv.Atoi(center.Port())
		}
		isCenter := false
		if centerPort == int(port) {
			for _, source := range portHosts {
				for _, name := range append([]string{source.Host}, splitLegacyList(source.BindMoreHost)...) {
					if strings.EqualFold(name, center.Hostname()) {
						isCenter = true
					}
				}
			}
		}
		if isCenter {
			var all []model.Hosts
			if err := optionalLegacyRows(db, &all, "global_host = ?", 0); err != nil {
				return evidence, err
			}
			pending := map[int]bool{}
			for _, source := range all {
				if source.START_STATUS != 0 || !accessgate.IsAccessEnabled(model.ParseAccessConfig(source.AccessJSON).Mode, enabled) {
					continue
				}
				for _, listen := range utils.ResolveHostListens(source) {
					if listen.Port != int(port) && listen.Port > 0 && listen.Port < 65536 && !wafpipeline.OwnsLegacyPort(uint16(listen.Port)) {
						pending[listen.Port] = true
					}
				}
			}
			if len(pending) > 0 {
				out.Blockers = append(out.Blockers, "该端口承载旧SSO认证中心；请先接管其余使用SSO的活动业务端口，再迁入认证中心，避免旧站点失去登录入口")
			}
		}
	}
	return evidence, nil
}
