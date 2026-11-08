// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package waftask

import (
	"HomeWaf/common/zlog"
	"HomeWaf/global"
	"HomeWaf/utils/wechat"
)

// TaskWechatAccessToken 初始化access token
func TaskWechatAccessToken() {
	if global.GWAF_NOTICE_ENABLE == false {
		return
	}
	zlog.Debug("TaskWechatAccessToken")
	wr, err := wechat.GetAppAccessToken("wx*********", "eb57************")
	if err != nil {
		zlog.Error("请求错误GetAppAccessToken")
	} else if wr.ErrCode != 0 {
		zlog.Error("Wechat Server:", wr.ErrMsg)
	} else {
		global.GCACHE_WECHAT_ACCESS = wr.AccessToken
		zlog.Debug("TaskWechatAccessToken获取到最新token:" + global.GCACHE_WECHAT_ACCESS)
	}

}
