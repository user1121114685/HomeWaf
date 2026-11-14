// Package channels implements direct notification providers. The channel catalogue
// describes the same settings used by validation and the management UI.
package channels

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/url"
	"strconv"
	"strings"

	"HomeWaf/utils"
)

//go:embed catalog.json
var catalogueJSON []byte

type Field struct {
	Key      string              `json:"key"`
	Label    string              `json:"label"`
	Kind     string              `json:"kind"`
	Required bool                `json:"required,omitempty"`
	Secret   bool                `json:"secret,omitempty"`
	Default  interface{}         `json:"default,omitempty"`
	Options  []string            `json:"options,omitempty"`
	Hint     string              `json:"hint,omitempty"`
	When     map[string][]string `json:"when,omitempty"`
}

type Definition struct {
	Type   string  `json:"type"`
	Name   string  `json:"name"`
	Hint   string  `json:"hint,omitempty"`
	Fields []Field `json:"fields"`
}

var catalogue = func() []Definition {
	var definitions []Definition
	if err := json.Unmarshal(catalogueJSON, &definitions); err != nil {
		panic(err)
	}
	return definitions
}()

func Catalogue() []Definition { return catalogue }

func DefinitionFor(kind string) *Definition {
	for i := range catalogue {
		if catalogue[i].Type == kind {
			return &catalogue[i]
		}
	}
	return nil
}

// Handles excludes SMTP and the existing fully customizable HTTP webhook.
func Handles(kind string) bool {
	return DefinitionFor(kind) != nil && kind != "email" && kind != "webhook"
}

type Config map[string]interface{}

func (c Config) String(key string) string {
	if value, ok := c[key].(string); ok {
		return value
	}
	if value, ok := c[key].(float64); ok {
		return strconv.FormatFloat(value, 'f', -1, 64)
	}
	return ""
}
func (c Config) Bool(key string) bool { value, _ := c[key].(bool); return value }
func (c Config) Int(key string) int   { value, _ := strconv.Atoi(c.String(key)); return value }
func (c Config) Default(key, fallback string) string {
	if v := c.String(key); v != "" {
		return v
	}
	return fallback
}

func decode(raw string) (Config, error) {
	if strings.TrimSpace(raw) == "" {
		return Config{}, nil
	}
	var c Config
	if err := json.Unmarshal([]byte(raw), &c); err != nil || c == nil {
		return nil, fmt.Errorf("渠道配置必须是 JSON 对象")
	}
	return c, nil
}

func active(field Field, c Config) bool {
	for key, values := range field.When {
		found := false
		for _, value := range values {
			if c.String(key) == value {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func Parse(kind, raw string) (Config, error) {
	d := DefinitionFor(kind)
	if d == nil {
		return nil, fmt.Errorf("不支持的通知渠道")
	}
	c, err := decode(raw)
	if err != nil {
		return nil, err
	}
	for _, f := range d.Fields {
		_, exists := c[f.Key]
		if f.Default != nil && (!exists || (f.Kind == "select" && c.String(f.Key) == "")) {
			c[f.Key] = f.Default
		}
	}
	for _, f := range d.Fields {
		if !active(f, c) {
			continue
		}
		v := c.String(f.Key)
		if f.Required && strings.TrimSpace(v) == "" {
			return nil, fmt.Errorf("%s不能为空", f.Label)
		}
		if value, exists := c[f.Key]; exists && value != nil {
			if f.Kind == "boolean" {
				if _, ok := value.(bool); !ok {
					return nil, fmt.Errorf("%s必须是开关值", f.Label)
				}
			} else {
				switch value.(type) {
				case string, float64:
				default:
					return nil, fmt.Errorf("%s格式错误", f.Label)
				}
			}
		}
		if v == "" {
			continue
		}
		if len(v) > 65536 {
			return nil, fmt.Errorf("%s过长", f.Label)
		}
		if f.Kind == "number" {
			n, e := strconv.ParseFloat(v, 64)
			if e != nil || math.IsNaN(n) || math.IsInf(n, 0) || n != math.Trunc(n) || n < 0 || n > 2147483647 {
				return nil, fmt.Errorf("%s必须是非负整数", f.Label)
			}
		}
		if len(f.Options) > 0 {
			found := false
			for _, option := range f.Options {
				if v == option {
					found = true
				}
			}
			if !found {
				return nil, fmt.Errorf("%s选项无效", f.Label)
			}
		}
		if f.Kind == "url" {
			if ok, _ := utils.PrecheckOutboundURL(v); !ok {
				return nil, fmt.Errorf("%s不是允许的 HTTP(S) 地址；内网服务请配置 security.outbound_allowed_hosts", f.Label)
			}
			u, e := url.Parse(v)
			if e != nil || u.User != nil || u.Fragment != "" {
				return nil, fmt.Errorf("%s不支持 URL 内嵌账号或片段", f.Label)
			}
		}
	}
	if kind == "socket" {
		if c.Int("port") < 1 || c.Int("port") > 65535 {
			return nil, fmt.Errorf("端口必须在 1–65535 之间")
		}
		host := c.String("address")
		if strings.ContainsAny(host, "/@?#[]\\") || strings.TrimSpace(host) != host {
			return nil, fmt.Errorf("主机地址只能填写域名或 IP")
		}
		if ok, _ := utils.PrecheckOutboundURL("http://" + net.JoinHostPort(host, c.String("port"))); !ok {
			return nil, fmt.Errorf("Socket 地址不被允许；内网服务请配置 security.outbound_allowed_hosts")
		}
		if c.String("method") == "MQTT" {
			if strings.ContainsAny(c.String("outMessageTopic"), "+#\x00") {
				return nil, fmt.Errorf("MQTT 发布主题不能包含通配符")
			}
			if c.String("response") != "" && c.String("inMessageTopic") == "" {
				return nil, fmt.Errorf("设置成功应答时必须填写 MQTT 接收主题")
			}
			if c.String("path") != "" && !strings.HasPrefix(c.String("path"), "/") {
				return nil, fmt.Errorf("WebSocket 路径必须以 / 开头")
			}
		}
	}
	if kind == "wechatwork_app" {
		if c.Int("agentID") < 1 {
			return nil, fmt.Errorf("AgentID 必须是正整数")
		}
		if c.String("toUser") == "" && c.String("toParty") == "" && c.String("toTag") == "" {
			return nil, fmt.Errorf("请至少指定一种企业微信接收对象")
		}
	}
	if kind == "dingtalk_app" {
		if count := len(splitIDs(c.String("userIds"))); count < 1 || count > 20 {
			return nil, fmt.Errorf("钉钉接收用户数量须为 1–20")
		}
	}
	if kind == "telegram" && strings.ContainsAny(c.String("apiToken"), "/?# \r\n") {
		return nil, fmt.Errorf("Telegram Bot Token 格式错误")
	}
	if kind == "bark" {
		if err := validateBarkEncryption(c); err != nil {
			return nil, err
		}
	}
	if c.String("messageCard") != "" {
		var card map[string]interface{}
		if json.Unmarshal([]byte(c.String("messageCard")), &card) != nil || card == nil {
			return nil, fmt.Errorf("自定义消息卡片必须是 JSON 对象")
		}
	}
	return c, nil
}

// MaskConfig never changes persisted data. URL fields containing tokens are also
// secrets, so list/detail responses cannot expose them.
func MaskConfig(kind, raw string) (string, map[string]bool) {
	d := DefinitionFor(kind)
	if d == nil || len(d.Fields) == 0 {
		return raw, nil
	}
	c, err := decode(raw)
	if err != nil {
		return "{}", nil
	}
	configured := map[string]bool{}
	for _, f := range d.Fields {
		if f.Secret {
			configured[f.Key] = c.String(f.Key) != ""
			delete(c, f.Key)
		}
	}
	b, _ := json.Marshal(c)
	return string(b), configured
}

// MergeSecrets is used before validation: an empty redacted value preserves the
// existing secret; the explicit clear sentinel clears it. Changing type never
// transfers credentials to a different provider.
func MergeSecrets(kind, submitted, current, clear string) (string, error) {
	d := DefinitionFor(kind)
	if d == nil || len(d.Fields) == 0 {
		return submitted, nil
	}
	c, err := decode(submitted)
	if err != nil {
		return "", err
	}
	old, err := decode(current)
	if err != nil {
		return "", err
	}
	for _, f := range d.Fields {
		if !f.Secret {
			continue
		}
		if c.String(f.Key) == clear {
			c[f.Key] = ""
		} else if c.String(f.Key) == "" {
			c[f.Key] = old.String(f.Key)
		}
	}
	b, err := json.Marshal(c)
	return string(b), err
}
