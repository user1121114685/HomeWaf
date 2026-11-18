package middleware

import (
	"HomeWaf/model"
	"HomeWaf/model/request"
	"HomeWaf/wafdb"
	"HomeWaf/wafdb/dialect"
	"testing"
)

func TestMCPUpgradeAddsPurposeWithoutGrantingExistingKeys(t *testing.T) {
	var previous dialect.DBDialect
	func() { defer func() { _ = recover() }(); previous = dialect.Get() }()
	dialect.Register(&dialect.SQLiteDialect{})
	t.Cleanup(func() { dialect.Register(previous) })
	db := openApiLocalTestDB(t)
	id, credential, err := openApiKeyService.AddApi(request.WafOPlatformKeyAddReq{KeyName: "existing-rest-key"})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Migrator().DropColumn(&model.OPlatformKey{}, "MCPAccess"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = wafdb.InitializeCoreSchema(db); err != nil {
			t.Fatal(err)
		}
	}
	if !db.Migrator().HasColumn(&model.OPlatformKey{}, "MCPAccess") {
		t.Fatal("existing installation missing MCP purpose column")
	}
	key, err := openApiKeyService.GetByApiKey(credential)
	if err != nil || key.Id != id || key.MCPAccess != "" {
		t.Fatal("schema upgrade changed or granted an existing credential", err)
	}
	if _, _, err = openApiKeyService.AddApi(request.WafOPlatformKeyAddReq{KeyName: "new-mcp-key", MCPAccess: "write", IPWhitelist: "192.0.2.0/24"}); err != nil {
		t.Fatal("upgraded installation cannot create MCP keys", err)
	}
}
