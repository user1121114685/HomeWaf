// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package waftask

import (
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model"
	"HomeWaf/internal/platform/common/uuid"
	"HomeWaf/internal/platform/common/wsmsg"
	"HomeWaf/internal/platform/common/zlog"
	"HomeWaf/internal/services/waf_service"
	"time"

	"github.com/coder/websocket"
)

// TaskDelayInfo 定时发送延迟信息
func TaskDelayInfo() {
	zlog.Debug("TaskDelayInfo")
	models, count, err := waf_service.WafDelayMsgServiceApp.GetAllList()
	if err == nil {
		if count > 0 {
			for i := 0; i < len(models); i++ {
				msg := models[i]

				cmdType := "Info"
				if msg.DelayType == "升级结果" {
					cmdType = "RELOAD_PAGE"
				}
				dataPacket := model.MsgDataPacket{
					MessageId:           uuid.GenUUID(),
					MessageType:         msg.DelayType,
					MessageData:         msg.DelayContent,
					MessageAttach:       nil,
					MessageDateTime:     time.Now().Format("2006-01-02 15:04:05"),
					MessageUnReadStatus: true,
				}
				//发送websocket：走会话管理器统一出口，内部按连接加锁 + 写超时；
				// 按每条连接各自的会话密钥组装报文。
				sendSuccess := global.GWebSocket.BroadcastBuild(websocket.MessageText, func(keyID string) ([]byte, error) {
					msgBytes, err := wsmsg.Build(keyID, dataPacket, cmdType)
					if err != nil {
						zlog.Debug("组装延迟消息报文错误", err)
					}
					return msgBytes, err
				})

				if sendSuccess > 0 {
					waf_service.WafDelayMsgServiceApp.DelApi(msg.Id)
				}

			}
		}
	}
}
