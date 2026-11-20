// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package waftask

import (
	"HomeWaf/internal/instance/global"
	"HomeWaf/internal/model/innerbean"
	"HomeWaf/internal/platform/common/zlog"
	"HomeWaf/internal/services/waf_service"
	"fmt"
)

// TaskStatusNotify 汇总通知
func TaskStatusNotify() {
	zlog.Debug("TaskStatusNotify")
	statHomeInfo, err := waf_service.WafStatServiceApp.StatHomeSumDayApi()
	if err == nil {
		noticeStr := fmt.Sprintf("今日访问量：%d 今天恶意访问量:%d 昨日恶意访问量:%d", statHomeInfo.VisitCountOfToday, statHomeInfo.AttackCountOfToday, statHomeInfo.AttackCountOfYesterday)

		serverName := global.GWAF_CUSTOM_SERVER_NAME
		if serverName == "" {
			serverName = "未命名服务器"
		}
		global.GQEQUE_MESSAGE_DB.Enqueue(innerbean.OperatorMessageInfo{
			BaseMessageInfo: innerbean.BaseMessageInfo{
				OperaType: "汇总通知",
				Server:    serverName,
			},
			OperaCnt: noticeStr,
		})
	} else {
		zlog.Error("TaskStatusNotifyerror", err)
	}

}
