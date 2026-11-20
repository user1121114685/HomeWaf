package wafcli

import (
	"HomeWaf/global"
	"HomeWaf/utils"
	"HomeWaf/utils/localca"
	"HomeWaf/wafconfig"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

func newConsoleInfoCommand(deps Deps) *cobra.Command {
	var wait time.Duration
	var showInitialPassword bool
	cmd := &cobra.Command{
		Use: "console-info", Short: "显示管理端访问地址和首次登录信息", GroupID: "service", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if wait < 0 || wait > 10*time.Minute {
				return fail(ExitUsage, errors.New("--wait 必须在 0 到 10m 之间"))
			}
			entry := ""
			if global.GWAF_SECURITY_ENTRY_ENABLE {
				entry = global.GWAF_SECURITY_ENTRY_PATH
			}
			if wait > 0 {
				ctx, cancel := context.WithTimeout(cmd.Context(), wait)
				defer cancel()
				// The mixed HTTP/TLS listener returns a redirect when HTTPS is required.
				// Do not follow it: a private certificate may not trust the loopback name.
				url := "http://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(global.GWAF_LOCAL_SERVER_PORT)) + "/" + entry
				if err := waitForConsole(ctx, url); err != nil {
					return fail(ExitUnavailable, fmt.Errorf("管理端尚未就绪，请检查 HomeWaf 服务和 logs 目录: %w", err))
				}
			}
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "HomeWaf console — open a server address reachable from your device:")
			urls := utils.ConsoleURLs(global.GWAF_LOCAL_SERVER_PORT, global.GWAF_SSL_ENABLE, entry, wafconfig.GetManagementDomain())
			for _, url := range urls {
				fmt.Fprintln(out, "  "+url)
			}
			if domain := wafconfig.GetManagementDomain(); domain != "" {
				fmt.Fprintf(out, "Only the configured domain is accepted: https://%s/%s\n", net.JoinHostPort(domain, strconv.Itoa(global.GWAF_LOCAL_SERVER_PORT)), entry)
			}
			if len(urls) == 0 {
				fmt.Fprintf(out, "  No network IP detected. Use the server IP with port %d.\n", global.GWAF_LOCAL_SERVER_PORT)
			}
			fmt.Fprintln(out, "Account: admin. Configured IP/domain access restrictions still apply.")
			paths := localca.DefaultPaths(filepath.Dir(deps.DataDir))
			if ca := localca.CASummary(paths); ca != nil && localca.IsIssuedByLocalCA(paths) {
				fmt.Fprintln(out, "HTTPS uses this instance's HomeWaf Local CA. Import its public root on your browsing device:")
				fmt.Fprintln(out, "  Root certificate: "+paths.CACert)
				fmt.Fprintln(out, "  SHA-256: "+ca.Fingerprint)
				if wafconfig.GetManagementDomain() == "" && !global.GWAF_SSL_FORCE_HTTPS {
					fmt.Fprintln(out, "First visit: use HTTP to download the root certificate, verify its fingerprint, then open HTTPS:")
					for _, url := range utils.ConsoleURLs(global.GWAF_LOCAL_SERVER_PORT, false, entry, "") {
						fmt.Fprintln(out, "  "+url)
					}
				}
			}
			if showInitialPassword {
				if err := printInitialPassword(out, filepath.Join(deps.DataDir, "initial_password.txt")); err != nil {
					return fail(ExitFailed, err)
				}
			}
			return nil
		},
	}
	cmd.Flags().DurationVar(&wait, "wait", 0, "等待管理端就绪，例如 90s")
	cmd.Flags().BoolVar(&showInitialPassword, "show-initial-password", false, "在当前终端显示初始口令文件（已改密时请使用当前口令）")
	return cmd
}

func waitForConsole(ctx context.Context, url string) error {
	client := &http.Client{
		Timeout:       2 * time.Second,
		Transport:     &http.Transport{Proxy: nil},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	defer client.CloseIdleConnections()
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 500 {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func printInitialPassword(out io.Writer, path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		fmt.Fprintln(out, "No initial password file. For an existing instance, use your current password.")
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取初始口令文件失败: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() > 16*1024 {
		return errors.New("初始口令文件必须是普通文件，且不得超过 16 KiB")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "Initial credentials (valid only until the password is changed):")
	_, err = fmt.Fprintln(out, string(data))
	return err
}
