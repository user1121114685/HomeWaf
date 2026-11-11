package waf_service

import (
	"HomeWaf/global"
	"HomeWaf/model"
	"HomeWaf/wafbot"
	"HomeWaf/wafpipeline"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"strconv"
	"strings"
)

func migrateLegacyRequestOptions(db *gorm.DB, h model.Hosts, c *wafpipeline.PortConfig, out *LegacyPortPreview, mode wafpipeline.Mode) error {
	if h.GUARD_STATUS == 1 {
		c.Policy.RuleSet.Callback = &wafpipeline.CallbackPolicy{Mode: wafpipeline.ModeOff, Networks: wafbot.LegacyCallbackNetworks()}
		out.Warnings = append(out.Warnings, "旧源码内置的支付回调IP表保留为关闭的端口例外；不再把历史IP自动视为可信身份。启用前需指定回调路径并核对来源；仅跳过后续攻击检测，不跳过黑名单、登录门或业务签名校验")
	}
	for _, raw := range []string{h.CsrfJSON, h.UploadSecurityJSON, h.AntiLeechJSON, h.CaptchaJSON, h.CookieSecurityJSON, h.ResponseCompressJSON} {
		if raw != "" && !json.Valid([]byte(raw)) {
			return errors.New("旧请求检查配置JSON无效，请先修复源配置")
		}
	}
	csrf := model.ParseCsrfConfig(h.CsrfJSON)
	upload := model.ParseUploadSecurityConfig(h.UploadSecurityJSON)
	c.Policy.RuleSet.CSRFOptions = &csrf
	c.Policy.RuleSet.UploadOptions = &upload
	for i := range c.Policy.RuleSet.Rules {
		r := &c.Policy.RuleSet.Rules[i]
		switch r.Category {
		case "csrf":
			r.Mode = wafpipeline.ModeOff
			if csrf.IsEnable == 1 {
				r.Mode = mode
			}
		case "upload":
			r.Mode = wafpipeline.ModeOff
			if upload.IsEnable == 1 {
				r.Mode = mode
			}
		}
	}
	var leech model.AntiLeechConfig
	if h.AntiLeechJSON != "" {
		if err := json.Unmarshal([]byte(h.AntiLeechJSON), &leech); err != nil {
			return err
		}
	}
	if leech.IsEnableAntiLeech == 1 {
		c.Policy.RuleSet.AntiLeech = &wafpipeline.AntiLeechPolicy{Mode: mode, Settings: leech}
	}
	challenge := model.ParseCaptchaConfig(h.CaptchaJSON)
	if challenge.IsEnableCaptcha == 1 || h.EmergencyMode == 1 {
		c.Policy.RuleSet.Challenge = wafpipeline.ChallengePolicy{Mode: mode, Scope: wafpipeline.Scope{ExcludePaths: splitLegacyList(string(challenge.ExcludeURLs))}}
		if h.EmergencyMode == 1 && challenge.IsEnableCaptcha != 1 {
			c.Policy.RuleSet.Challenge.Until = h.EmergencyUntil
		}
		out.Warnings = append(out.Warnings, "旧验证码/紧急模式迁为端口独立的内嵌签名Cookie轻量挑战，不复用旧通行Cookie；接口客户端未通过时返回429，旧验证码外观与算法不保留")
	}
	if csrf.IsEnable == 1 {
		for _, name := range append([]string{h.Host}, splitLegacyList(h.BindMoreHost)...) {
			if name != "" && name != "*" {
				c.Policy.RuleSet.CSRFOrigins = append(c.Policy.RuleSet.CSRFOrigins, c.Protocol+"://"+name+portOriginSuffix(c.Port, c.Protocol, name))
			}
		}
	}
	c.Policy.Network.Response = wafpipeline.ResponsePolicy{Cookie: model.ParseCookieSecurityConfig(h.CookieSecurityJSON), Compression: model.ParseResponseCompressConfig(h.ResponseCompressJSON), DefaultEncoding: h.DefaultEncoding}
	if model.ParseHostsDefense(h.DEFENSE_JSON).DEFENSE_SENSITIVE == 1 && db.Migrator().HasTable(&model.Sensitive{}) {
		var rows []model.Sensitive
		if err := db.Where("user_code = ? AND tenant_id = ?", h.USER_CODE, h.Tenant_ID).Order("id").Limit(5001).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) > 5000 {
			return errors.New("敏感词超过端口5000条上限")
		}
		if len(rows) > 0 {
			c.Policy.RuleSet.Sensitive = &wafpipeline.SensitivePolicy{Mode: mode, Replacement: global.GWAF_HTTP_SENSITIVE_REPLACE_STRING, Rules: rows}
		}
	}
	return nil
}
func portOriginSuffix(port uint16, protocol, host string) string {
	if strings.Contains(host, ":") || protocol == "https" && port == 443 || protocol == "http" && port == 80 {
		return ""
	}
	return ":" + strconv.Itoa(int(port))
}
