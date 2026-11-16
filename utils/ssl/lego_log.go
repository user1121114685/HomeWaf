// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package ssl

import (
	"HomeWaf/common/zlog"
	"HomeWaf/internal/acmelog"
	"fmt"
	"strings"

	legolog "github.com/go-acme/lego/v4/log"
)

// lego 自己有一套日志，默认写 stderr。SamWaf 以服务/后台方式运行时没人收 stderr，
// 等于整个 ACME 交互过程（拿授权、选校验方式、CA 校验结果、失败原文）全部丢掉了，
// 排查时只剩最后一句笼统的错误。这里把它接到 zlog，让每一步都落进日志文件：
//
//	[域名] acme: Obtaining bundled SAN certificate
//	[域名] AuthURL: https://acme-v02.api.letsencrypt.org/acme/authz-v3/xxxx
//	[域名] acme: Trying to solve HTTP-01
//	[域名] The server validated our request
type legoLogger struct{}

func (legoLogger) Print(args ...any) {
	acmeInfo(acmeLogPrefix + fmt.Sprint(args...))
}

func (legoLogger) Println(args ...any) {
	acmeInfo(acmeLogPrefix + strings.TrimRight(fmt.Sprintln(args...), "\n"))
}

func (legoLogger) Printf(format string, args ...any) {
	acmeInfo(acmeLogPrefix + fmt.Sprintf(format, args...))
}

// Fatal 系列一律降级成 Error。
//
// lego 的 StdLogger 接口带 Fatal，而它的默认实现（标准库 log.Logger）会 os.Exit(1)——
// 一个第三方库的日志调用不该有权力把整个 WAF 进程结束掉。
// 当前版本的 lego 运行时路径并不调用 Fatal（只有它自带的代码生成器在用），
// 这里是防御性写法，避免将来升级 lego 时被动踩坑。
func (legoLogger) Fatal(args ...any) {
	acmeError(acmeLogPrefix + fmt.Sprint(args...))
}

func (legoLogger) Fatalln(args ...any) {
	acmeError(acmeLogPrefix + strings.TrimRight(fmt.Sprintln(args...), "\n"))
}

func (legoLogger) Fatalf(format string, args ...any) {
	acmeError(acmeLogPrefix + fmt.Sprintf(format, args...))
}

// init 只做变量赋值、不写日志，所以不依赖 zlog 是否已经初始化；
// 真正产生日志要等到证书申请/续期时，那时 zlog 早已就绪。
func init() {
	legolog.Logger = legoLogger{}
}

func acmeInfo(message string)  { logACME("info", message) }
func acmeWarn(message string)  { logACME("warn", message) }
func acmeError(message string) { logACME("error", message) }
func logACME(level, message string) {
	message = acmelog.Sanitize(message)
	switch level {
	case "error":
		zlog.Error(message)
	case "warn":
		zlog.Warn(message)
	default:
		zlog.Info(message)
	}
	acmelog.Emit(level, message)
}
