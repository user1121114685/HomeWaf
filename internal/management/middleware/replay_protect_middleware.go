// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package middleware

import (
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model/common/response"
	"HomeWaf/internal/model/enums"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	replayTimeHeader  = "X-Request-Time"
	replayNonceHeader = "X-Request-Id"
	replayTimeWindow  = 5 * time.Minute
	replayNonceTTL    = 10 * time.Minute
)

func isLogExportDownloadPath(path string) bool {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	return len(parts) == 6 && parts[0] == "api" && parts[1] == "v1" &&
		parts[2] == "waflog" && parts[3] == "exports" && parts[4] != "" && parts[5] == "download"
}

func ReplayProtect() gin.HandlerFunc {
	return func(c *gin.Context) {
		if global.CurrentSettings().GCONFIG_ENABLE_REPLAY_PROTECT == 0 {
			c.Next()
			return
		}
		// 开放平台 API Key 请求跳过（Auth 中间件已在前面设置此标记）
		if v, exists := c.Get("is_openapi"); exists && v == true {
			c.Next()
			return
		}
		// WebSocket 升级跳过（Token 鉴权已足够）
		if c.GetHeader("Upgrade") == "websocket" {
			c.Next()
			return
		}

		// 校验 X-Request-Time
		tsStr := c.GetHeader(replayTimeHeader)
		nonce := c.GetHeader(replayNonceHeader)
		if tsStr == "" || nonce == "" {
			// 短效票据下载通过 window.open 发起，浏览器只能把时间戳与 nonce 放在查询串。
			if isLogExportDownloadPath(c.Request.URL.Path) {
				if tsStr == "" {
					tsStr = c.Query(replayTimeHeader)
				}
				if nonce == "" {
					nonce = c.Query(replayNonceHeader)
				}
			}
		}
		if tsStr == "" {
			response.FailWithMessage("请求缺少时间标头", c)
			c.Abort()
			return
		}
		ts, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			response.FailWithMessage("请求时间格式错误", c)
			c.Abort()
			return
		}
		diff := time.Now().Sub(time.Unix(ts, 0))
		if diff < -replayTimeWindow || diff > replayTimeWindow {
			response.FailWithMessage("请求已过期，请重试", c)
			c.Abort()
			return
		}

		// 校验 X-Request-Id（Nonce）
		if nonce == "" || len(nonce) < 16 || len(nonce) > 128 {
			response.FailWithMessage("请求标识无效", c)
			c.Abort()
			return
		}
		cacheKey := enums.CACHE_REPLAY_NONCE + nonce
		if global.GCACHE_WAFCACHE.IsKeyExist(cacheKey) {
			response.FailWithMessage("重复请求，已拒绝", c)
			c.Abort()
			return
		}
		global.GCACHE_WAFCACHE.SetWithTTl(cacheKey, true, replayNonceTTL)

		c.Next()
	}
}
