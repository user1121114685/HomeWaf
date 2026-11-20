package router

import (
	"HomeWaf/internal/model/enums"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestManagementAddressRequiresSystemAdministrator(t *testing.T) {
	for _, role := range []string{enums.ROLE_SECURITY_ADMIN, enums.ROLE_AUDIT_ADMIN, enums.ROLE_SYSTEM_ADMIN, enums.ROLE_SUPER_ADMIN} {
		r := gin.New()
		(&WafVpConfigRouter{}).InitWafVpConfigRouter(r.Group("", func(c *gin.Context) { c.Set("userRole", role); c.Next() }))
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/v1/vipconfig/managementAddress", strings.NewReader("invalid-json"))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		forbidden := strings.Contains(w.Body.String(), `"code":-403`)
		if forbidden != (role == enums.ROLE_SECURITY_ADMIN || role == enums.ROLE_AUDIT_ADMIN) {
			t.Fatalf("incorrect management certificate permission: %s", role)
		}
	}
}
