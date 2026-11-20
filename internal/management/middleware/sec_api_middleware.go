// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package middleware

import (
	"HomeWaf/internal/model/common/response"
	"HomeWaf/internal/platform/wafsec"
	"bytes"
	"errors"
	"github.com/gin-gonic/gin"
	"io"
	"mime"
	"net/http"
)

// SecApi 鉴权中间件
func SecApi() gin.HandlerFunc {
	return func(c *gin.Context) {
		isOpenAPI := c.GetBool("is_openapi")
		keyID := c.GetHeader(response.HeaderKeyID)
		if c.FullPath() == "/api/v1/ws" {
			keyID = c.Query("keyid")
		}
		if !isOpenAPI {
			session, err := wafsec.OpenTransportSession(keyID)
			if err != nil {
				response.Rehandshake(c)
				return
			}
			c.Set(response.TransportSessionKey, session)
		}
		// Multipart uploads are plaintext: preserve streaming and existing 200 MiB
		// application uploads instead of buffering them during transport decryption.
		if c.ContentType() == "multipart/form-data" {
			_, params, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
			if err != nil || params["boundary"] == "" || !isManagementFileUpload(c.FullPath()) {
				c.AbortWithStatus(http.StatusUnsupportedMediaType)
				return
			}
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 210<<20)
			c.Next()
			return
		}
		// Bound ciphertext before buffering/decryption, including chunked public
		// requests with unknown Content-Length.
		const maxTransportBody = 32 << 20
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxTransportBody)
		if c.FullPath() == "/api/v1/wafhost/host/diagnostic" {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
		}
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": "请求内容超过允许大小"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read request body"})
			c.Abort()
			return
		}
		if !isOpenAPI && len(bodyBytes) != 0 {
			plain, id, err := wafsec.TransportDecrypt(string(bodyBytes))
			if err != nil || id != keyID {
				response.Rehandshake(c)
				return
			}
			bodyBytes = plain
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		c.Next()
	}
}

func isManagementFileUpload(path string) bool {
	switch path {
	case "/api/v1/iplocation/upload", "/api/v1/license/checklicense",
		"/api/v1/application/app/upload", "/api/v1/application/app/upgrade":
		return true
	}
	return false
}
