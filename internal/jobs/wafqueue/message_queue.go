// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package wafqueue

import (
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/model/innerbean"
	"HomeWaf/internal/platform/common/uuid"
	"HomeWaf/internal/platform/common/wsmsg"
	"HomeWaf/internal/platform/common/zlog"
	"HomeWaf/internal/platform/utils"
	"HomeWaf/internal/services/waf_service"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

/*
*
处理消息队列信息
*/
func ProcessMessageDequeEngine() {
	// 启动订阅级通知聚合器（按「订阅 × 去重key」分桶，窗口取各订阅自己的配置）
	waf_service.NotifyAggregatorApp.StartFlushLoop(global.GWAF_QUEUE_SHUTDOWN_SIGNAL)

	for {
		select {
		case <-global.GWAF_QUEUE_SHUTDOWN_SIGNAL:
			zlog.Info("消息队列处理协程收到关闭信号，正在退出...")
			return
		default:
			for !global.GQEQUE_MESSAGE_DB.Empty() {
				popFront, ok := global.GQEQUE_MESSAGE_DB.Dequeue()
				if !ok {
					zlog.Error("来得信息未空")
					continue
				}

				// 处理不同类型的消息
				switch msg := popFront.(type) {
				case innerbean.RuleMessageInfo:
					handleRuleMessage(msg)
				case innerbean.OperatorMessageInfo:
					handleOperatorMessage(msg)
				case innerbean.UserLoginMessageInfo:
					handleUserLoginMessage(msg)
				case innerbean.AttackInfoMessageInfo:
					handleAttackInfoMessage(msg)
				case innerbean.WeeklyReportMessageInfo:
					handleWeeklyReportMessage(msg)
				case innerbean.SSLExpireMessageInfo:
					handleSSLExpireMessage(msg)
				case innerbean.SystemErrorMessageInfo:
					handleSystemErrorMessage(msg)
				case innerbean.IPBanMessageInfo:
					handleIPBanMessage(msg)
				case innerbean.AccessMessageInfo:
					handleAccessMessage(msg)
				case innerbean.LogExportReadyMessageInfo:
					//导出任务产物就绪：只通知任务号，下载由用户从任务列表发起
					sendToWebSocket("日志导出任务", msg.Msg, map[string]interface{}{"task_id": msg.TaskID}, "LOG_EXPORT_READY")
				case innerbean.UpdateResultMessageInfo:
					//升级结果
					sendToWebSocket("升级结果", msg.Msg, nil, "Info")
				case innerbean.OpResultMessageInfo:
					//操作实时结果
					sendToWebSocket("信息通知", msg.Msg, nil, "Info")
				case innerbean.SystemStatsData:
					//系统统计数据
					sendToWebSocket("系统统计信息", "", msg, "SystemStats")
				}

			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// ========== 通知防雪崩总闸 ==========
//
// 历史包袱说明（issue #822）：
// 这里原本是唯一的频率控制点，用「规则原文」当 key 做 1min→5min→15min 递增冷却。
// 两个问题让它形同虚设：
//   1. 攻击方变换 payload，规则原文就变，每个新串都算"首次出现→立即发送"，冷却被绕过；
//   2. 所有渠道共用一把锁，没法做到"飞书只收严重告警、邮件收全量"。
//
// 现在真正的频率控制下沉到了订阅维度（internal/services/waf_service/waf_notify_throttle.go），
// 这里只保留一道粗粒度总闸，作用仅剩一个：CC 期间事件洪水时保护队列与 goroutine，
// 阈值刻意放宽，正常业务量不会碰到它。

const (
	notifyGateWindow    = time.Second // 限速窗口
	notifyGateMaxPerSec = 50          // 每种消息类型每秒最多放行多少条事件
)

type notifyGateBucket struct {
	windowStart time.Time
	count       int
	dropped     int
}

var (
	notifyGateMu      sync.Mutex
	notifyGateBuckets = make(map[string]*notifyGateBucket)
)

// checkCanSend 防雪崩总闸：按消息类型限速，超出部分直接丢弃（只记 debug 日志，不落库）
func checkCanSend(messageType string) bool {
	now := time.Now()

	notifyGateMu.Lock()
	defer notifyGateMu.Unlock()

	b, ok := notifyGateBuckets[messageType]
	if !ok {
		b = &notifyGateBucket{windowStart: now}
		notifyGateBuckets[messageType] = b
	}
	if now.Sub(b.windowStart) >= notifyGateWindow {
		if b.dropped > 0 {
			zlog.Debug(fmt.Sprintf("通知总闸丢弃事件: 类型=%s 丢弃=%d", messageType, b.dropped))
		}
		b.windowStart = now
		b.count = 0
		b.dropped = 0
	}
	if b.count >= notifyGateMaxPerSec {
		b.dropped++
		return false
	}
	b.count++
	return true
}

// ========== 各类消息处理函数（保持队列+WebSocket方式，集成新的通知系统） ==========

// NotifySoftwareUpdate runs on the release-check task, outside the traffic message queue.
func NotifySoftwareUpdate(current, latest string) {
	event := waf_service.SoftwareUpdateEvent(current, latest)
	sendToWebSocket(event.MessageTypeName, event.DefaultContent, nil, "UPDATE_AVAILABLE")
	waf_service.WafNotifySenderServiceApp.SendEvent(event)
}

func sendConsoleAndSubscription(msg any, label, command string) string {
	event := waf_service.WafNotifySenderServiceApp.BuildNotifyEvent(msg)
	content := waf_service.PlainNotifyContent(event.DefaultContent)
	sendToWebSocket(label, content, nil, command)
	waf_service.WafNotifySenderServiceApp.SendEvent(event)
	return content
}

func handleRuleMessage(msg innerbean.RuleMessageInfo) {
	if !checkCanSend(model.MSG_TYPE_RULE_TRIGGER) {
		return
	}
	sendConsoleAndSubscription(msg, "命中保护规则", "Info")
	if global.GWAF_NOTICE_ENABLE {
		utils.NotifyHelperApp.SendRuleInfo(msg)
	}
}

func handleOperatorMessage(msg innerbean.OperatorMessageInfo) {
	if !checkCanSend(model.MSG_TYPE_OPERATION_NOTICE) {
		return
	}
	sendConsoleAndSubscription(msg, msg.OperaType, "Info")
	utils.NotifyHelperApp.SendNoticeInfo(msg)
}

func handleUserLoginMessage(msg innerbean.UserLoginMessageInfo) {
	if msg.Abnormal {
		sendConsoleAndSubscription(msg, "登录来源变化", "Warning")
		return
	}
	sendConsoleAndSubscription(msg, "用户登录", "Info")
}

func handleAttackInfoMessage(msg innerbean.AttackInfoMessageInfo) {
	if checkCanSend(model.MSG_TYPE_ATTACK_INFO) {
		sendConsoleAndSubscription(msg, "攻击告警", "Info")
	}
}

func handleAccessMessage(msg innerbean.AccessMessageInfo) {
	level := "Info"
	if msg.Abnormal {
		level = "Warning"
	}
	sendConsoleAndSubscription(msg, "统一访问认证", level)
}

func handleWeeklyReportMessage(msg innerbean.WeeklyReportMessageInfo) {
	sendConsoleAndSubscription(msg, "WAF周报", "Info")
}

func handleSSLExpireMessage(msg innerbean.SSLExpireMessageInfo) {
	sendConsoleAndSubscription(msg, "SSL证书过期提醒", "Info")
}

func handleSystemErrorMessage(msg innerbean.SystemErrorMessageInfo) {
	sendConsoleAndSubscription(msg, "系统错误", "Info")
}

func handleIPBanMessage(msg innerbean.IPBanMessageInfo) {
	if !checkCanSend(model.MSG_TYPE_IP_BAN) {
		return
	}
	content := sendConsoleAndSubscription(msg, "IP封禁通知", "Info")
	if strings.Contains(msg.OperaType, "主机远程登录爆破") {
		sendToWebSocket("主机防爆破封禁", content, msg, "HostGuard")
	}
}

// sendToWebSocket 统一的 WebSocket 发送函数。
// 写入必须走 global.GWebSocket.BroadcastBuild：它按连接加锁串行化并带写超时，
// 直接对裸连接 WriteMessage 会与 ping 回显、定时任务撞成 concurrent write panic。
func sendToWebSocket(messageType, messageData string, messageAttach interface{}, cmdType string) {
	// 在线表尚未建好时直接丢弃：本协程可能早于初始化跑起来，对 nil 调方法会 panic。
	// 正常启动顺序下不会走到这里，留着是防止将来调整启动顺序时再踩一次。
	if global.GWebSocket == nil {
		zlog.Debug("WebSocket 在线表未就绪，跳过本条通知", messageType)
		return
	}
	dataPacket := model.MsgDataPacket{
		MessageId:           uuid.GenUUID(),
		MessageType:         messageType,
		MessageData:         messageData,
		MessageAttach:       messageAttach,
		MessageDateTime:     time.Now().Format("2006-01-02 15:04:05"),
		MessageUnReadStatus: true,
	}
	// 按每条连接各自的会话密钥组装报文。
	global.GWebSocket.BroadcastBuild(websocket.MessageText, func(keyID string) ([]byte, error) {
		msgBytes, err := wsmsg.Build(keyID, dataPacket, cmdType)
		if err != nil {
			zlog.Debug("组装websocket报文错误", err)
		}
		return msgBytes, err
	})
}
