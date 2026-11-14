// Package dnsproviders describes the providers shipped by the pinned lego module.
// catalogue.json contains upstream documentation; see LEGO-LICENSE and README.md.
package dnsproviders

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"strings"
)

type Field struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Advanced    bool   `json:"advanced"`
}
type Provider struct {
	Code          string   `json:"code"`
	Name          string   `json:"name"`
	Aliases       []string `json:"aliases"`
	Documentation string   `json:"documentation"`
	Fields        []Field  `json:"fields"`
	Instructions  string   `json:"instructions"`
	Example       string   `json:"example"`
	Unavailable   string   `json:"unavailable,omitempty"`
}
type Catalogue struct {
	Version   string     `json:"version"`
	Providers []Provider `json:"providers"`
}

//go:embed catalogue.json
var catalogueJSON []byte
var catalogue Catalogue
var byCode = map[string]Provider{}

func init() {
	if err := json.Unmarshal(catalogueJSON, &catalogue); err != nil {
		panic(err)
	}
	for _, p := range catalogue.Providers {
		byCode[p.Code] = p
		for _, alias := range p.Aliases {
			byCode[alias] = p
		}
	}
}

func List() Catalogue                     { return catalogue }
func Lookup(code string) (Provider, bool) { p, ok := byCode[code]; return p, ok }
func ValidateProvider(code string) error {
	p, ok := Lookup(code)
	if !ok {
		return fmt.Errorf("不支持的 DNS 服务商：%s", code)
	}
	if p.Unavailable != "" {
		return fmt.Errorf("%s", p.Unavailable)
	}
	return nil
}

func AllowedKey(code, key string) bool {
	p, ok := Lookup(code)
	if !ok {
		return false
	}
	for _, f := range p.Fields {
		if key == f.Name {
			return true
		}
	}
	return false
}

// ValidateValues never interpolates a credential value into an error.
// Required/alternative authentication modes are validated by the real lego constructor.
func ValidateValues(code string, values map[string]string) error {
	if err := ValidateProvider(code); err != nil {
		return err
	}
	for key, value := range values {
		if err := ValidateField(code, key, value); err != nil {
			return err
		}
	}
	if code == "hurricane" {
		_, err := ParseHurricaneTokens(values["HURRICANE_TOKENS"])
		return err
	}
	return nil
}

// ValidateField permits saving each field independently, including optional settings.
func ValidateField(code, key, value string) error {
	if err := ValidateProvider(code); err != nil {
		return err
	}
	if !AllowedKey(code, key) {
		return fmt.Errorf("%s 不支持配置项 %s，请参照该服务商的配置说明", code, key)
	}
	if strings.TrimSpace(value) == "" || value == "****" || strings.ContainsRune(value, 0) {
		return fmt.Errorf("配置项 %s 的值不能为空或使用脱敏占位符", key)
	}
	if code == "hurricane" && key == "HURRICANE_TOKENS" {
		_, err := ParseHurricaneTokens(value)
		return err
	}
	return nil
}

var domainLabel = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

func ValidDomain(domain string) bool {
	if len(domain) > 253 || net.ParseIP(domain) != nil {
		return false
	}
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return false
	}
	for _, label := range labels {
		if !domainLabel.MatchString(label) {
			return false
		}
	}
	return true
}

// Keys omit both the wildcard and the _acme-challenge label, as required by lego.
func ParseHurricaneTokens(value string) (map[string]string, error) {
	result := map[string]string{}
	for _, pair := range strings.Split(value, ",") {
		domain, token, ok := strings.Cut(strings.TrimSpace(pair), ":")
		if !ok || !ValidDomain(domain) || strings.HasPrefix(domain, "_acme-challenge.") || strings.TrimSpace(token) == "" || strings.ContainsAny(token, " \t\r\n:") {
			return nil, fmt.Errorf("HURRICANE_TOKENS 请填写 域名:DDNS Key，多项用逗号分隔；域名不含 *. 或 _acme-challenge.，例如 example.com:token")
		}
		if _, exists := result[domain]; exists {
			return nil, fmt.Errorf("HURRICANE_TOKENS 中域名重复：%s；根域名与泛域名共用一项", domain)
		}
		result[domain] = token
	}
	return result, nil
}
