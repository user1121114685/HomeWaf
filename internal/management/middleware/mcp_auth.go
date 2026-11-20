package middleware

import (
	"HomeWaf/internal/model"
	"HomeWaf/internal/platform/common/zlog"
	"HomeWaf/internal/platform/utils"
	"HomeWaf/internal/platform/wafsec"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type mcpIdentityKey struct{}
type mcpGrantKey struct{}
type mcpIdentity struct {
	key        model.OPlatformKey
	request    *http.Request
	credential string
	clientIP   string
}
type mcpGrant struct {
	keyID, method, path, tool string
	write                     bool
}

// MCPAuth authenticates every HTTP request. MCP credentials have no REST authority.
func MCPAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		if origins := c.Request.Header.Values("Origin"); len(origins) > 0 {
			origin, err := url.Parse(origins[0])
			if len(origins) != 1 || err != nil || (origin.Scheme != "https" && origin.Scheme != "http") || origin.User != nil || !strings.EqualFold(origin.Host, c.Request.Host) || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "MCP origin is not allowed"})
				return
			}
		}
		parts := strings.Fields(c.GetHeader("Authorization"))
		if len(c.Request.Header.Values("Authorization")) != 1 || len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || c.GetHeader("X-API-Key") != "" || c.Request.URL.RawQuery != "" {
			c.Header("WWW-Authenticate", `Bearer realm="HomeWaf MCP"`)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Use a dedicated MCP key in the Authorization: Bearer header"})
			return
		}
		c.Set("mcp_transport", true)
		key, ok := ValidateOpenApiKey(c, parts[1])
		if !ok {
			c.Abort()
			return
		}
		if key.MCPAccess != "read" && key.MCPAccess != "write" {
			c.Header("WWW-Authenticate", `Bearer realm="HomeWaf MCP"`)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "请使用 MCP 专用密钥"})
			return
		}
		// Leave room for the JSON-RPC envelope around a valid 1 MiB service draft.
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20)
		identity := mcpIdentity{key: key, request: c.Request.Clone(c.Request.Context()), credential: parts[1], clientIP: utils.GetManageClientIP(c)}
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), mcpIdentityKey{}, identity))
		c.Next()
	}
}

func allowMCPKey(c *gin.Context, key model.OPlatformKey) bool {
	if key.MCPAccess != "read" && key.MCPAccess != "write" {
		return false
	}
	if c.GetBool("mcp_transport") {
		return true
	}
	grant, ok := c.Request.Context().Value(mcpGrantKey{}).(mcpGrant)
	return ok && grant.keyID == key.Id && grant.method == c.Request.Method && grant.path == c.Request.URL.Path && (!grant.write || key.MCPAccess == "write")
}

func MCPAccess(ctx context.Context) string {
	identity, _ := ctx.Value(mcpIdentityKey{}).(mcpIdentity)
	return identity.key.MCPAccess
}

// RecordMCPResult receives the exact, already redacted text handed to the client.
// Keep one audit row per tool call, including failures before internal dispatch.
func RecordMCPResult(ctx context.Context, method, path, tool string, start time.Time, status int, result string, isError bool) {
	identity, ok := ctx.Value(mcpIdentityKey{}).(mcpIdentity)
	if !ok {
		return
	}
	outcome := `{"isError":false}`
	if isError {
		outcome = `{"isError":true}`
	}
	entry := model.OPlatformLog{
		ApiKeyId: identity.key.Id, KeyName: identity.key.KeyName, ClientIP: identity.clientIP,
		RequestMethod: method, RequestPath: path, RequestBody: "MCP tool: " + tool,
		StatusCode: status, Duration: time.Since(start).Milliseconds(), TimeStr: start.Format("2006-01-02 15:04:05"),
		ResponseBody: outcome,
	}
	const maxMCPLogBytes = 47 * 1024
	if len(result) > maxMCPLogBytes {
		truncated := strings.ToValidUTF8(result[:maxMCPLogBytes], "")
		encoded, _ := json.Marshal(map[string]any{"truncated": true, "result_prefix": truncated})
		result = string(encoded)
	}
	var err error
	entry.MCPResult, err = wafsec.DataEncrypt(result)
	if err != nil {
		zlog.Error("MCP 返回内容加密失败", "tool", tool, "error", err.Error())
	}
	if err := openApiLogService.AddLog(entry); err != nil {
		zlog.Error("MCP 调用审计写入失败", "tool", tool, "error", err.Error())
	}
	openApiKeyService.IncrCallCount(identity.key.Id)
}

// NewMCPDispatchRequest grants one catalogued operation inside this process only.
// The ordinary management router rechecks credential revocation, IP policy and limits.
func NewMCPDispatchRequest(ctx context.Context, method, path, query, tool string, write bool, body []byte) (*http.Request, error) {
	identity, ok := ctx.Value(mcpIdentityKey{}).(mcpIdentity)
	if !ok || (identity.key.MCPAccess != "read" && identity.key.MCPAccess != "write") || (write && identity.key.MCPAccess != "write") {
		return nil, errors.New("MCP key does not permit this operation")
	}
	if !strings.HasPrefix(path, "/api/v1/") || strings.ContainsAny(path, "?#\\") {
		return nil, errors.New("invalid MCP operation path")
	}
	grant := mcpGrant{keyID: identity.key.Id, method: method, path: path, tool: tool, write: write}
	r := identity.request.Clone(context.WithValue(ctx, mcpGrantKey{}, grant))
	r.Method = method
	r.URL = &url.URL{Path: path, RawQuery: query}
	r.RequestURI = r.URL.RequestURI()
	r.Header = identity.request.Header.Clone()
	r.Header.Del("Authorization")
	r.Header.Del("Cookie")
	r.Header.Del("Token")
	r.Header.Del("Mcp-Session-Id")
	r.Header.Set("X-API-Key", identity.credential)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	return r, nil
}
