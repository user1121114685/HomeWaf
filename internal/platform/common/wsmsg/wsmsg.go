// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package wsmsg

import (
	"HomeWaf/internal/model"
	"HomeWaf/internal/platform/wafsec"
	"encoding/json"
)

// Build 按目标连接的会话密钥标识组装一条 WebSocket 推送报文。
func Build(keyID string, dataPacket model.MsgDataPacket, cmdType string) ([]byte, error) {
	msgBody, err := json.Marshal(dataPacket)
	if err != nil {
		return nil, err
	}

	payload, err := wafsec.TransportEncrypt(keyID, msgBody)
	if err != nil {
		return nil, err
	}

	return json.Marshal(model.MsgPacket{
		MsgCode:       "200",
		MsgDataPacket: payload,
		MsgCmdType:    cmdType,
	})
}
