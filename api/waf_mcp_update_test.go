package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestMCPUpdateDiagnosticsRemainReadOnly(t *testing.T) {
	wanted := map[string]string{"version_info": "/sysinfo/version", "update_check": "/sysinfo/checkversion", "update_progress": "/sysinfo/updateprogress"}
	for _, operation := range MCPOperations() {
		if suffix, ok := wanted[operation.Name]; ok {
			if !operation.ReadOnly || operation.Confirm || operation.Method != http.MethodGet || !strings.HasSuffix(operation.Path, suffix) {
				t.Fatalf("unsafe diagnostic operation: %+v", operation)
			}
			delete(wanted, operation.Name)
		}
		if strings.HasSuffix(operation.Path, "/sysinfo/update") || strings.HasSuffix(operation.Path, "/sysinfo/rollback") {
			t.Fatal("MCP diagnostics unexpectedly gained executable replacement authority")
		}
	}
	if len(wanted) != 0 {
		t.Fatalf("missing diagnostics: %v", wanted)
	}
}
