// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package api

import (
	"HomeWaf/common/gwebsocket"
	"HomeWaf/common/zlog"
	"HomeWaf/global"
	"HomeWaf/model"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
)

type WafWebSocketApi struct {
}

func (w *WafWebSocketApi) WebSocketMessageApi(c *gin.Context) {
	origin := c.GetHeader("Origin")
	var originPatterns []string
	if origin != "" {
		parsed, parseErr := url.Parse(origin)
		allowed := parseErr == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && strings.EqualFold(parsed.Host, c.Request.Host)
		for _, entry := range strings.Split(global.CurrentSettings().GCONFIG_CORS_ALLOW_ORIGINS, ",") {
			if strings.EqualFold(strings.TrimRight(strings.TrimSpace(entry), "/"), strings.TrimRight(origin, "/")) {
				allowed = true
			}
		}
		if !allowed || parseErr != nil {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		originPatterns = []string{parsed.Host}
	}

	//获取用户账号：
	tokenStr := c.GetHeader("Sec-WebSocket-Protocol")
	tokenInfo := wafTokenInfoService.GetInfoByAccessToken(tokenStr)

	//本次连接声明的传输会话密钥（v2 客户端才带；旧客户端为空，推送回落 legacy）。
	//WebSocket 建连没法带自定义头，只能走查询参数。
	keyID := c.Query("keyid")

	//升级get请求为webSocket协议
	ws, err := websocket.Accept(c.Writer, c.Request, &websocket.AcceptOptions{Subprotocols: []string{tokenStr}, OriginPatterns: originPatterns})
	if err != nil {
		zlog.Error("websocketinit", err)
		return
	}

	if tokenInfo.LoginAccount == "" {
		//写入ws数据（此时连接还没登记到管理器，只有当前协程在写）
		msgBytes, _ := json.Marshal(model.MsgPacket{
			MsgCode:    "-999",
			MsgCmdType: "Info",
		})
		writeContext, cancel := context.WithTimeout(c.Request.Context(), gwebsocket.WriteWait)
		err = ws.Write(writeContext, websocket.MessageText, msgBytes)
		cancel()
		zlog.Debug("无鉴权信息，请检查")
		ws.CloseNow() // 未鉴权连接必须显式关闭，否则连接一直挂着不释放
		return
	}

	// 读超时靠心跳续期：前端每 30s 发一次 ping，服务端也会主动发 Ping 帧，
	// 只要有一路活着就会刷新 deadline；都没了说明连接已死，读取直接报错退出。
	ws.SetReadLimit(64 * 1024)

	// 生成用户标识和会话ID
	userKey := tokenInfo.BaseOrm.Tenant_ID + tokenInfo.BaseOrm.USER_CODE + tokenInfo.LoginAccount
	sessionID := global.GWebSocket.AddWebSocketWithKey(userKey, keyID, ws)

	zlog.Debug("WebSocket连接建立，用户: " + userKey + ", 会话ID: " + sessionID)

	// 服务端主动心跳：探活半开连接，同时给读超时续期
	stopPing := make(chan struct{})
	go func() {
		ticker := time.NewTicker(gwebsocket.PingPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-stopPing:
				return
			case <-ticker.C:
				if err := global.GWebSocket.PingSession(sessionID); err != nil {
					global.GWebSocket.CloseSession(sessionID)
					return
				}
			}
		}
	}()

	defer func() {
		close(stopPing)
		// 只关闭当前会话的连接
		global.GWebSocket.CloseSession(sessionID)
		zlog.Debug("WebSocket连接已关闭，会话ID: " + sessionID)
	}()

	for {
		//读取ws中的数据（每条连接只有这一个读协程）
		mt, message, err := ws.Read(c.Request.Context())
		if err != nil {
			break
		}

		zlog.Debug("WebSocket message received, session: " + sessionID)
		if string(message) == "ping" {
			message = []byte("pong")
		}

		//写入ws数据：统一走会话管理器，内部按连接加锁，避免与队列/定时任务的广播并发写
		if err = global.GWebSocket.SendToSession(sessionID, mt, message); err != nil {
			zlog.Debug("WebSocket回写失败，退出消息循环，会话ID: " + sessionID)
			break
		}
	}
}
