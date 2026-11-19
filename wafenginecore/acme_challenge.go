// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package wafenginecore

import (
	"HomeWaf/global"
	"HomeWaf/utils"
	"HomeWaf/wafacme"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
)

// isACMEChallengePath 判断是否是 ACME(HTTP-01) 证书校验请求。
//
// cleanPath 必须是已经 path.Clean + 转小写的路径，rawPath 是原始的 r.URL.Path。
// 两个参数缺一不可：
//   - 只看 rawPath：/.well-known/acme-challenge/../../admin 会命中前缀被放行，
//     而点号段原样转发给后端，后端按 RFC 3986 归一化成 /admin —— 整个网关就废了
//   - 只看 cleanPath：/foo/../.well-known/acme-challenge/x 归一化后会命中前缀，
//     虽然目标确实落在 challenge 目录里，但这种写法只可能是刻意构造的探测
//
// 抽成独立函数是为了让回归测试能直接钉住这个判定本身，
// 而不是去测 path.Clean 的语义——后者即使有人把实现改回 rawPath 也照样是绿的。
func isACMEChallengePath(cleanPath, rawPath string) bool {
	if rawPath != path.Clean(rawPath) || cleanPath != strings.ToLower(rawPath) || !strings.HasPrefix(rawPath, global.GSSL_HTTP_CHANGLE_PATH) {
		return false
	}
	return utils.IsValidChallengeFile(strings.TrimPrefix(rawPath, global.GSSL_HTTP_CHANGLE_PATH))
}

// acmeChallengeToken 从请求里取出合法的挑战 token，取不到返回空串。
//
// 判定顺序是刻意排的：全部是纯 CPU 的字符串/正则操作，排在任何磁盘或内存表访问之前。
// /.well-known/ 是互联网上被扫烂的路径，非法请求必须在最便宜的地方就被打掉。
func acmeChallengeToken(r *http.Request) string {
	// CA 只会用 GET（HEAD 容错），其余方法一律不是校验请求
	if r.Method != http.MethodGet && r.Method != http.MethodHead || r.URL == nil {
		return ""
	}
	if r.URL.RawQuery != "" || r.URL.ForceQuery || r.URL.EscapedPath() != r.URL.Path || len(r.Header.Values("Upgrade")) > 0 {
		return ""
	}
	cleanPath := strings.ToLower(path.Clean(r.URL.Path))
	if !isACMEChallengePath(cleanPath, r.URL.Path) {
		return ""
	}
	// Only the exact canonical path is eligible for the local exception.
	token := strings.TrimPrefix(r.URL.Path, global.GSSL_HTTP_CHANGLE_PATH)
	if !utils.IsValidChallengeFile(token) {
		return ""
	}
	return token
}

// loadChallengeContent 取挑战文件内容，两层来源：内存注册表 → 磁盘。
//
// 防刷的闸门全在这里：
//  1. 门闩关闭（没有进行中的挑战）时立即返回，一次系统调用都不做 —— 99.99% 的时间都是这条路径
//  2. 内存注册表命中即返回，O(1) 且无系统调用
//  3. 只有"门闩开着但内存没有"才会读盘，而这只在蓝绿升级双 Worker 并存时才正常发生，
//     所以额外加了每秒读盘次数上限
func loadChallengeContent(hostCode, token string) (string, bool) {
	if !wafacme.GateOpen() {
		return "", false
	}
	if keyAuth, ok := wafacme.Lookup(hostCode, token); ok {
		return keyAuth, true
	}
	if !wafacme.AllowDiskRead() {
		return "", false
	}
	return readChallengeFile(hostCode, token)
}

// challengeFileOpen 是唯一的挑战文件打开入口，抽成变量是为了让测试能替换它，
// 直接断言"门闩关闭时一次磁盘 IO 都不发生"——这是防刷设计的核心，
// 用行为断言（返回值）替代不了，必须数真实的打开次数。
var challengeFileOpen = os.Open

// readChallengeFile 读磁盘上的挑战文件。
// 路径由 acmeChallengeFilePath 统一拼装，token 已经过白名单校验，不存在穿越空间。
func readChallengeFile(hostCode, token string) (string, bool) {
	filePath := acmeChallengeFilePath(hostCode, token)
	f, err := challengeFileOpen(filePath)
	if err != nil {
		return "", false
	}
	defer f.Close()

	// 限长读取：真实 keyAuth 只有 87 字节左右，
	// 设上限是为了万一那个目录被塞进大文件时不至于整块读进内存
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > wafacme.MaxChallengeFileSize() {
		return "", false
	}
	buf, err := io.ReadAll(io.LimitReader(f, wafacme.MaxChallengeFileSize()+1))
	if err != nil || int64(len(buf)) > wafacme.MaxChallengeFileSize() {
		return "", false
	}
	content := string(buf)
	if content == "" {
		return "", false
	}
	return content, true
}

// acmeChallengeFilePath 返回挑战文件应该在的绝对路径。
//
// 拼法必须与申请时传给 lego 的 savePath 一致（见 sslorder.go 的
// GetCurrentDir()/data/vhost/<HostCode>），否则就是"写在一处、读在另一处"。
// 统一成一个函数，避免调用点各拼各的。
func acmeChallengeFilePath(hostCode, token string) string {
	return utils.GetCurrentDir() + "/data/vhost/" + hostCode + "/.well-known/acme-challenge/" + token
}
