// HomeWaf modification (2026-11-14): isolated DNS credential scopes.
package wafssl

import (
	"HomeWaf/internal/dnsproviders"
	"HomeWaf/model"
	"HomeWaf/service/waf_service"
)

// WithDNSCredentials restores the environment on success, error and panic.
// Credentials are decrypted only inside the worker, never logged.
func WithDNSCredentials(order model.SslOrder, run func() error) error {
	if order.ApplyMethod != "dns01" {
		return run()
	}
	values, err := waf_service.LoadDNSCredentials(order.ApplyDns, order.PrivateGroupName)
	if err != nil {
		return err
	}
	return dnsproviders.WithEnvironment(order.ApplyDns, values, run)
}
