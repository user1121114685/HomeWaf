package middleware

import (
	"HomeWaf/internal/model/request"
	"HomeWaf/internal/storage/wafdb"
	"testing"
)

func TestMCPAccessIsOptInAndSurvivesRestart(t *testing.T) {
	db := openApiLocalTestDB(t)
	for _, scope := range []string{"", "write"} {
		id, credential, err := openApiKeyService.AddApi(request.WafOPlatformKeyAddReq{KeyName: "key-" + scope, MCPAccess: scope, IPWhitelist: "192.0.2.0/24"})
		if err != nil {
			t.Fatal(err)
		}
		if err := wafdb.InitializeCoreSchema(db); err != nil {
			t.Fatal(err)
		}
		key, err := openApiKeyService.GetByApiKey(credential)
		if err != nil || key.Id != id || key.MCPAccess != scope {
			t.Fatalf("credential scope after restart: expected %q, err=%v", scope, err)
		}
	}
}
