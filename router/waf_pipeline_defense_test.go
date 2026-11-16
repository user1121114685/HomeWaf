package router

import (
	"HomeWaf/enums"
	"HomeWaf/middleware"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPipelineDefenseRouteAuthenticationAndRoles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	endpoints := []struct{ method, path string }{
		{"GET", "/overview"},
		{"GET", "/stats?port=5000"},
		{"GET", "/logs?port=5000"},
		{"GET", "/state?port=5000"},
		{"POST", "/state/manage"},
		{"GET", "/defense"},
		{"GET", "/defense/capabilities"},
		{"POST", "/ports/access"},
		{"GET", "/honeypot"},
		{"GET", "/honeypot/logs"},
		{"POST", "/honeypot/save"},
	}
	for _, tc := range []struct {
		name, role string
		auth       bool
		code       int
	}{
		{name: "unauthenticated", auth: true, code: -999},
		{name: "system admin", role: enums.ROLE_SYSTEM_ADMIN, code: -403},
		{name: "audit admin", role: enums.ROLE_AUDIT_ADMIN, code: -403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			group := r.Group("")
			if tc.auth {
				group.Use(middleware.Auth())
			} else {
				group.Use(func(c *gin.Context) { c.Set("userRole", tc.role) })
			}
			InitPipelineRouter(group)
			for _, endpoint := range endpoints {
				recorder := httptest.NewRecorder()
				r.ServeHTTP(recorder, httptest.NewRequest(endpoint.method, "/api/v1/pipeline"+endpoint.path, nil))
				var reply struct {
					Code int `json:"code"`
				}
				if err := json.Unmarshal(recorder.Body.Bytes(), &reply); err != nil {
					t.Fatal(err)
				}
				if reply.Code != tc.code {
					t.Errorf("%s %s: got %s, want code %d", endpoint.method, endpoint.path, recorder.Body.String(), tc.code)
				}
			}
		})
	}
	// Authorized requests reach validation without mutating a service or database.
	for _, role := range []string{enums.ROLE_SUPER_ADMIN, enums.ROLE_SECURITY_ADMIN} {
		t.Run(role, func(t *testing.T) {
			r := gin.New()
			group := r.Group("", func(c *gin.Context) { c.Set("userRole", role) })
			InitPipelineRouter(group)
			for _, endpoint := range []struct{ method, path string }{{"GET", "/defense"}, {"POST", "/ports/access"}, {"POST", "/honeypot/save"}} {
				recorder := httptest.NewRecorder()
				r.ServeHTTP(recorder, httptest.NewRequest(endpoint.method, "/api/v1/pipeline"+endpoint.path, nil))
				var reply struct {
					Code int `json:"code"`
				}
				if err := json.Unmarshal(recorder.Body.Bytes(), &reply); err != nil {
					t.Fatal(err)
				}
				if reply.Code != -1 {
					t.Errorf("%s %s: authorized request did not reach validation: %s", endpoint.method, endpoint.path, recorder.Body.String())
				}
			}
		})
	}
}
