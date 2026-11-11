package wafenginecore

import (
	"HomeWaf/global"
	"net/http"
)

// Direct detector tests may have no request; serving requests always carry a
// pinned configuration installed by ServeHTTP and inherited by the proxy.
func requestSettings(r *http.Request) global.RuntimeSettings {
	if r == nil {
		return global.CurrentSettings()
	}
	return global.SettingsForContext(r.Context())
}
