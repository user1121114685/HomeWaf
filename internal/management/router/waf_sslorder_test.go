package router

import (
	"net/http/httptest"
	"strings"
	"testing"

	"HomeWaf/internal/model/enums"
	"github.com/gin-gonic/gin"
)

func TestZeroSSLAccountRequiresSystemAdministrator(t *testing.T) {
	for _, role := range []string{enums.ROLE_SECURITY_ADMIN, enums.ROLE_AUDIT_ADMIN, enums.ROLE_SYSTEM_ADMIN, enums.ROLE_SUPER_ADMIN} {
		r := gin.New()
		group := r.Group("", func(c *gin.Context) { c.Set("userRole", role); c.Next() })
		(&SslOrderRouter{}).InitSslOrderRouter(group)
		response := httptest.NewRecorder()
		request := httptest.NewRequest("POST", "/api/v1/wafhost/sslorder/zerossl/account", strings.NewReader("invalid-json"))
		request.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(response, request)
		forbidden := strings.Contains(response.Body.String(), `"code":-403`)
		if forbidden != (role == enums.ROLE_SECURITY_ADMIN || role == enums.ROLE_AUDIT_ADMIN) {
			t.Fatalf("incorrect credential permission for %s: %s", role, response.Body.String())
		}
	}
}

func TestSSLOrderRetryAndLogsRequireSystemAdministrator(t *testing.T) {
	for _, endpoint := range []struct{ method, path string }{{"POST", "/api/v1/wafhost/sslorder/retry"}, {"GET", "/api/v1/wafhost/sslorder/logs"}} {
		for _, role := range []string{enums.ROLE_SECURITY_ADMIN, enums.ROLE_AUDIT_ADMIN, enums.ROLE_SYSTEM_ADMIN, enums.ROLE_SUPER_ADMIN} {
			r := gin.New()
			(&SslOrderRouter{}).InitSslOrderRouter(r.Group("", func(c *gin.Context) { c.Set("userRole", role); c.Next() }))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(endpoint.method, endpoint.path, strings.NewReader("{}")))
			forbidden := strings.Contains(w.Body.String(), `"code":-403`)
			if forbidden != (role == enums.ROLE_SECURITY_ADMIN || role == enums.ROLE_AUDIT_ADMIN) {
				t.Fatalf("incorrect permission for %s %s: %s", role, endpoint.path, w.Body.String())
			}
		}
	}
}
