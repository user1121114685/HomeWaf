package dnsproviders

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
)

// lego and several SDKs read the environment lazily. Keep the scope for the
// entire issuance/renewal, including Present, propagation and CleanUp.
var environmentMu sync.Mutex
var currentSecrets atomic.Value

// Redact also covers encoded credentials in SDK errors/HTTP URLs.
func Redact(message string) string {
	if values, ok := currentSecrets.Load().([]string); ok {
		for _, value := range values {
			if len(value) >= 4 {
				message = strings.ReplaceAll(message, value, "[REDACTED]")
				message = strings.ReplaceAll(message, url.QueryEscape(value), "[REDACTED]")
			}
		}
	}
	return message
}

func WithEnvironment(code string, values map[string]string, run func() error) error {
	if err := ValidateValues(code, values); err != nil {
		return err
	}
	environmentMu.Lock()
	defer environmentMu.Unlock()
	var secrets []string
	for _, value := range values {
		secrets = append(secrets, value)
	}
	if code == "hurricane" {
		tokens, _ := ParseHurricaneTokens(values["HURRICANE_TOKENS"])
		for _, token := range tokens {
			secrets = append(secrets, token)
		}
	}
	currentSecrets.Store(secrets)
	defer currentSecrets.Store([]string{})
	type savedValue struct {
		value   string
		present bool
	}
	saved := map[string]savedValue{}
	for _, p := range catalogue.Providers {
		for _, f := range p.Fields {
			// Also clear lego's file indirections so another group cannot supply credentials.
			for _, key := range []string{f.Name, f.Name + "_FILE"} {
				if _, exists := saved[key]; !exists {
					v, ok := os.LookupEnv(key)
					saved[key] = savedValue{v, ok}
				}
			}
		}
	}
	defer func() {
		for key, value := range saved {
			if value.present {
				_ = os.Setenv(key, value.value)
			} else {
				_ = os.Unsetenv(key)
			}
		}
	}()
	for key := range saved {
		if err := os.Unsetenv(key); err != nil {
			return fmt.Errorf("无法清理 DNS 配置项 %s", key)
		}
	}
	for key, value := range values {
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("无法加载 DNS 配置项 %s", key)
		}
	}
	if err := run(); err != nil {
		return errors.New(Redact(err.Error()))
	}
	return nil
}
