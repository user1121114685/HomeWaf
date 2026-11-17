package wafcli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

var updateClientFactory = newLocalClient

func newUpdateCommand() *cobra.Command {
	var wait time.Duration
	var version, channel string
	var check, status bool
	cmd := &cobra.Command{Use: "update", Aliases: []string{"upgrade"}, GroupID: "maintain", Short: "通过本机认证 API 更新，并等待自动平滑切换完成", Args: cobra.NoArgs,
		Example: "  homewaf apikey init\n  homewaf update --check\n  homewaf update --wait 10m\n  homewaf update --status -o json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if wait <= 0 || wait > 30*time.Minute {
				return fail(ExitUsage, errors.New("--wait 必须在 0 到 30m 之间"))
			}
			if channel != "official" && channel != "github" {
				return fail(ExitUsage, errors.New("无效更新渠道"))
			}
			if version != "latest" && !semver.IsValid("v"+strings.TrimPrefix(version, "v")) {
				return fail(ExitUsage, errors.New("--version 必须是 latest 或 vX.Y.Z"))
			}
			client, err := updateClientFactory(0)
			if err != nil {
				return err
			}
			defer client.http.CloseIdleConnections()
			ctx, cancel := context.WithTimeout(cmd.Context(), wait)
			defer cancel()
			if status {
				var progress map[string]any
				if err := client.call(ctx, http.MethodGet, "/api/v1/sysinfo/updateprogress", nil, &progress); err != nil {
					return err
				}
				return writeResult(cmd, progress, fmt.Sprintf("更新状态: %v；阶段: %v；错误: %v", progress["state"], progress["stage"], progress["error"]))
			}
			var info struct {
				Version    string `json:"version"`
				Target     string `json:"version_new"`
				NeedUpdate bool   `json:"need_update"`
				Allowed    *bool  `json:"self_update_allowed"`
			}
			if err := client.call(ctx, http.MethodGet, "/api/v1/sysinfo/checkversion", nil, &info); err != nil {
				return err
			}
			if check {
				return writeResult(cmd, info, fmt.Sprintf("当前版本: %s；可用版本: %s；需要更新: %v", info.Version, info.Target, info.NeedUpdate))
			}
			if !info.NeedUpdate {
				if version != "latest" && semver.Compare("v"+strings.TrimPrefix(version, "v"), "v"+strings.TrimPrefix(info.Version, "v")) != 0 {
					return fail(ExitFailed, errors.New("指定版本不是当前可用更新；不会降级或替换运行程序"))
				}
				return writeResult(cmd, map[string]any{"ok": true, "updated": false, "version": info.Version}, "当前已是最新版本，无需更新。")
			}
			if info.Allowed != nil && !*info.Allowed {
				return fail(ExitFailed, errors.New("当前实例不允许应用内升级；容器请使用安装脚本的 --docker 升级镜像"))
			}
			if !semver.IsValid(info.Target) {
				return fail(ExitFailed, errors.New("服务端返回了无效的目标版本"))
			}
			if channel == "official" && semver.Prerelease(info.Target) != "" {
				return fail(ExitFailed, errors.New("测试版本需要显式指定 --channel github"))
			}
			if version != "latest" && semver.Compare("v"+strings.TrimPrefix(version, "v"), info.Target) != 0 {
				return fail(ExitFailed, fmt.Errorf("当前可用版本为 %s，与指定版本不同，未启动更新", info.Target))
			}
			if err := client.call(ctx, http.MethodGet, "/api/v1/sysinfo/update?channel="+url.QueryEscape(channel), nil, nil); err != nil {
				return fmt.Errorf("%w；不要盲目重复提交，可先执行 homewaf update --status 查看结果", err)
			}
			if outputFormat != "json" {
				fmt.Fprintf(cmd.OutOrStdout(), "已受理更新 %s → %s，等待自动平滑重启完成…\n", info.Version, info.Target)
			}
			if err := waitForUpdatedVersion(ctx, client, info.Target); err != nil {
				return err
			}
			return writeResult(cmd, map[string]any{"ok": true, "updated": true, "from_version": info.Version, "version": info.Target}, "更新完成，新版本 "+info.Target+" 已接管。")
		},
	}
	cmd.Flags().DurationVar(&wait, "wait", 10*time.Minute, "等待更新和自动平滑重启的时间")
	cmd.Flags().StringVar(&version, "version", "latest", "期望目标版本；不匹配时不启动更新")
	cmd.Flags().StringVar(&channel, "channel", "official", "更新渠道：official 或 github")
	cmd.Flags().BoolVar(&check, "check", false, "只检查是否存在新版本")
	cmd.Flags().BoolVar(&status, "status", false, "只读取当前升级进度")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "输出格式：text 或 json")
	return cmd
}

func waitForUpdatedVersion(ctx context.Context, client *apiClient, target string) error {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		var current struct {
			Version string `json:"version"`
		}
		if err := client.call(ctx, http.MethodGet, "/api/v1/sysinfo/version", nil, &current); err == nil && semver.IsValid(current.Version) && semver.Compare(current.Version, target) == 0 {
			return nil
		}
		var progress struct {
			State string `json:"state"`
			Error string `json:"error"`
		}
		if err := client.call(ctx, http.MethodGet, "/api/v1/sysinfo/updateprogress", nil, &progress); err == nil && (progress.State == "failed" || progress.State == "canceled") {
			return fail(ExitFailed, fmt.Errorf("更新未完成（%s）: %s", progress.State, progress.Error))
		}
		select {
		case <-ctx.Done():
			return fail(ExitUnavailable, errors.New("等待新版本接管超时；请查看 update --status，勿盲目重试"))
		case <-ticker.C:
		}
	}
}
