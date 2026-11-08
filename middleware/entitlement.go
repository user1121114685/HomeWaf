package middleware

import (
	"HomeWaf/wafreg"
	"github.com/gin-gonic/gin"
	"net/http"
)

// RequireEntitlement is applied to a paid route only after a feature is assigned
// to a published product tier. Existing basic routes are not silently paywalled.
func RequireEntitlement(feature string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !wafreg.HasFeature(feature) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 403, "msg": "当前授权不包含此功能或已到期"})
			return
		}
		c.Next()
	}
}
