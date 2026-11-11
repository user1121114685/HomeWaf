package waf_service

import (
	"HomeWaf/model"
	"HomeWaf/wafpipeline"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"strings"
	"time"
	"unicode/utf8"
)

func (s *WafPipelineService) migrateLegacyTamper(db *gorm.DB, hosts []model.Hosts, c *wafpipeline.PortConfig, out *LegacyPortPreview, mode wafpipeline.Mode) error {
	if !db.Migrator().HasTable(&model.TamperRule{}) {
		return nil
	}
	for _, host := range hosts {
		if host.TamperJSON != "" && !json.Valid([]byte(host.TamperJSON)) {
			return errors.New("旧防篡改配置无效")
		}
		cfg := model.ParseTamperConfig(host.TamperJSON)
		if cfg.IsEnable != 1 {
			continue
		}
		var rows []model.TamperRule
		if err := db.Omit("baseline_content").Where("host_code = ? AND is_enable = ?", host.Code, 1).Order("id").Limit(129).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows)+len(c.Policy.Network.Response.Tamper) > 128 {
			return errors.New("防篡改规则超过端口128条上限")
		}
		for _, source := range rows {
			names := append([]string{host.Host}, splitLegacyList(host.BindMoreHost)...)
			if host.Host == "*" {
				names = nil
			}
			rule := wafpipeline.TamperRule{ID: source.Id, Path: source.Url, Hosts: names, Mode: mode, Action: cfg.Action, IgnoreQuery: source.IgnoreQuery == 1, MaxBytes: min(cfg.MaxSizeKB*1024, 1<<20)}
			if source.BaselineStatus == 1 {
				var payload struct{ BaselineContent []byte }
				if err := db.Model(&model.TamperRule{}).Select("baseline_content").Where("id = ? AND length(baseline_content) <= ?", source.Id, 1<<20).Take(&payload).Error; err != nil {
					return errors.New("旧防篡改基线超过1MiB或不可读取")
				}
				source.BaselineContent = payload.BaselineContent
				sum := sha256.Sum256(source.BaselineContent)
				if len(source.BaselineContent) > 1<<20 || hex.EncodeToString(sum[:]) != source.BaselineHash {
					return errors.New("旧防篡改基线超过1MiB或哈希不符")
				}
				body, charset, err := s.normalizeTamperSource(source.BaselineContent, source.ContentType, c.Policy.Network.Response.DefaultEncoding)
				if err != nil {
					return err
				}
				baseline := wafpipeline.NewTamperBaseline(rule, body, source.ContentType, charset)
				baseline.LearnedAt = max(0, time.Time(source.UPDATE_TIME).Unix())
				rule.LearnedAt = baseline.LearnedAt
				if out.secrets.TamperBaselines == nil {
					out.secrets.TamperBaselines = map[string]wafpipeline.TamperBaseline{}
				}
				out.secrets.TamperBaselines[rule.ID] = baseline
			}
			c.Policy.Network.Response.Tamper = append(c.Policy.Network.Response.Tamper, rule)
		}
	}
	if len(c.Policy.Network.Response.Tamper) > 0 {
		out.Warnings = append(out.Warnings, "防篡改规则和已学习基线复制到端口；基线加密保存、每端口总计1MiB，新增学习使用有界异步队列，仅完整200响应参与。请求者不会因上游内容篡改受处罚")
	}
	return wafpipeline.ValidateTamperBaselines(out.secrets.TamperBaselines)
}
func (s *WafPipelineService) normalizeTamperSource(body []byte, contentType, encoding string) ([]byte, string, error) {
	if s.NormalizeBaseline != nil {
		return s.NormalizeBaseline(body, contentType, encoding)
	}
	if utf8.Valid(body) && (encoding == "" || encoding == "auto" || strings.EqualFold(encoding, "utf-8")) && (!strings.Contains(strings.ToLower(contentType), "charset=") || strings.Contains(strings.ToLower(contentType), "charset=utf-8")) {
		return body, "utf-8", nil
	}
	return nil, "", errors.New("防篡改基线编码处理器未就绪")
}
