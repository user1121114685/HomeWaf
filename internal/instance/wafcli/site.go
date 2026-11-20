package wafcli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"text/tabwriter"
	"time"

	"HomeWaf/internal/protection/wafpipeline"
	"github.com/spf13/cobra"
)

// 测试通过工厂注入本地模拟管理端。
var siteClientFactory = newLocalClient

func newSiteCommand() *cobra.Command {
	var wait time.Duration
	cmd := &cobra.Command{Use: "site <command>", Short: "通过本机管理端管理站点", GroupID: "auto", Example: "  homewaf site apply -f site.json --wait 60s"}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return fail(ExitUsage, fmt.Errorf("unknown command %s", args[0]))
		}
		return nil
	}
	cmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "text", "输出格式：text或json")
	cmd.PersistentFlags().DurationVar(&wait, "wait", 0, "连接管理端的最长等待时间，例如60s")
	cmd.Args = func(cmd *cobra.Command, args []string) error {
		return nil
	}
	run := func(fn func(*cobra.Command, *apiClient, []string) error) func(*cobra.Command, []string) error {
		return func(c *cobra.Command, args []string) error {
			if wait < 0 {
				return fail(ExitUsage, errors.New("--wait不能为负数"))
			}
			a, err := siteClientFactory(wait)
			if err != nil {
				return err
			}
			defer a.http.CloseIdleConnections()
			return fn(c, a, args)
		}
	}
	list := &cobra.Command{Use: "list", Short: "列出全部站点", Args: cobra.NoArgs, Example: "  homewaf site list -o json", RunE: run(runSiteList)}
	get := &cobra.Command{Use: "get <port>", Short: "导出服务配置（省略密钥）", Args: cobra.ExactArgs(1), Example: "  homewaf site get 8080 > site.json", RunE: run(runSiteGet)}
	var file string
	apply := &cobra.Command{Use: "apply -f <file|->", Short: "创建或合并修改站点，相同配置跳过", Args: cobra.NoArgs, Example: "  homewaf site apply -f site.json --wait 60s\n  homewaf site apply -f - -o json", RunE: run(func(c *cobra.Command, a *apiClient, args []string) error {
		items, err := readSiteInput(file, c.InOrStdin())
		if err != nil {
			return err
		}
		return runSiteApply(c, a, items)
	})}
	apply.Flags().StringVarP(&file, "file", "f", "", "站点JSON文件，-表示标准输入")
	_ = apply.MarkFlagRequired("file")
	cmd.AddCommand(list, get, apply)
	for _, action := range []string{"delete", "start", "stop"} {
		action := action
		cmd.AddCommand(&cobra.Command{Use: action + " <port>", Short: map[string]string{"delete": "删除站点", "start": "启动站点", "stop": "停止站点"}[action], Args: cobra.ExactArgs(1), Example: "  homewaf site " + action + " 8080", RunE: run(func(c *cobra.Command, a *apiClient, args []string) error { return runSiteAction(c, a, action, args[0]) })})
	}
	return cmd
}

func (a *apiClient) list(ctx context.Context) ([]wafpipeline.PortConfig, error) {
	var sites []wafpipeline.PortConfig
	err := a.call(ctx, http.MethodGet, portsAPI, nil, &sites)
	return sites, err
}

func resolveSite(ctx context.Context, a *apiClient, ref string) (wafpipeline.PortConfig, error) {
	port, err := sitePort(ref)
	if err != nil {
		return wafpipeline.PortConfig{}, err
	}
	sites, err := a.list(ctx)
	if err != nil {
		return wafpipeline.PortConfig{}, err
	}
	for _, cfg := range sites {
		if cfg.Port == port {
			return cfg, nil
		}
	}
	return wafpipeline.PortConfig{}, fail(ExitNotFound, fmt.Errorf("未找到端口 %d 的服务", port))
}

func runSiteList(c *cobra.Command, a *apiClient, _ []string) error {
	sites, err := a.list(c.Context())
	if err != nil {
		return err
	}
	var out strings.Builder
	w := tabwriter.NewWriter(&out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "PORT\tNAME\tPROTOCOL\tLISTEN\tUPSTREAM")
	for _, cfg := range sites {
		fmt.Fprintf(w, "%d\t%s\t%s\t%t\t%s\n", cfg.Port, cfg.Name, cfg.Protocol, cfg.ListenEnabled, cfg.Upstream)
	}
	w.Flush()
	return writeResult(c, map[string]any{"ok": true, "total": len(sites), "items": sites}, strings.TrimSuffix(out.String(), "\n"))
}

func runSiteGet(c *cobra.Command, a *apiClient, args []string) error {
	cfg, err := resolveSite(c.Context(), a, args[0])
	if err != nil {
		return err
	}
	if outputFormat == "json" {
		return writeResult(c, map[string]any{"ok": true, "site": cfg}, "")
	}
	enc := json.NewEncoder(c.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(cfg)
}

func (a *apiClient) save(ctx context.Context, cfg wafpipeline.PortConfig) error {
	return a.call(ctx, http.MethodPost, portsAPI+"/save", map[string]any{"config": cfg}, nil)
}

func runSiteAction(c *cobra.Command, a *apiClient, action, ref string) error {
	cfg, err := resolveSite(c.Context(), a, ref)
	if err != nil {
		return err
	}
	result := "unchanged"
	if action == "delete" {
		err = a.call(c.Context(), http.MethodPost, portsAPI+"/delete", map[string]any{"port": cfg.Port}, nil)
		result = "deleted"
	} else if enabled := action == "start"; enabled != cfg.ListenEnabled {
		cfg.ListenEnabled = enabled
		err = a.save(c.Context(), cfg)
		result = "updated"
	}
	if err != nil {
		return err
	}
	return writeResult(c, map[string]any{"ok": true, "action": result, "port": cfg.Port}, fmt.Sprintf("%s %d", result, cfg.Port))
}

type applyResult struct {
	Port   uint16 `json:"port"`
	Action string `json:"action"`
	Error  string `json:"error,omitempty"`
}

func errorCode(err error) int {
	var e *ExitError
	if errors.As(err, &e) {
		return e.Code
	}
	return ExitFailed
}

func runSiteApply(c *cobra.Command, a *apiClient, items []json.RawMessage) error {
	existing, err := a.list(c.Context())
	if err != nil {
		return err
	}
	current := make(map[uint16]wafpipeline.PortConfig, len(existing))
	for _, cfg := range existing {
		current[cfg.Port] = cfg
	}
	configs := make([]wafpipeline.PortConfig, len(items))
	for i, item := range items {
		var id struct {
			Port uint16 `json:"port"`
		}
		if err := json.Unmarshal(item, &id); err != nil {
			return fail(ExitUsage, err)
		}
		cfg, found := current[id.Port]
		if found {
			cfg = wafpipeline.CloneConfig(cfg)
		} else {
			cfg = wafpipeline.PortConfig{Schema: wafpipeline.SchemaVersion, Protocol: "http", Policy: wafpipeline.DefaultPolicy()}
		}
		if err := wafpipeline.StrictDecode(item, &cfg); err != nil {
			return fail(ExitUsage, err)
		}
		if err := wafpipeline.ValidateConfig(cfg, 0); err != nil {
			return fail(ExitUsage, fmt.Errorf("端口 %d：%w", id.Port, err))
		}
		if !found && (cfg.Revision != 0 || cfg.ListenEnabled || !cfg.Policy.Defense.Enabled) {
			return fail(ExitUsage, fmt.Errorf("端口 %d：新建服务应选择异常处置策略，以 revision=0、listen_enabled=false 创建后再启动", id.Port))
		}
		if found && cfg.Revision != current[id.Port].Revision {
			return fail(ExitFailed, fmt.Errorf("端口 %d 的配置已更新，请重新获取配置", id.Port))
		}
		configs[i] = cfg
	}
	results := make([]applyResult, 0, len(items))
	var out strings.Builder
	firstCode := 0
	for _, cfg := range configs {
		old, found := current[cfg.Port]
		result := applyResult{Port: cfg.Port, Action: "unchanged"}
		if !found || !reflect.DeepEqual(old, cfg) {
			result.Action = "updated"
			if !found {
				result.Action = "created"
			}
			if err := a.save(c.Context(), cfg); err != nil {
				result.Action, result.Error = "failed", err.Error()
				if firstCode == 0 {
					firstCode = errorCode(err)
				}
			}
		}
		results = append(results, result)
		fmt.Fprintf(&out, "%s %d %s\n", result.Action, cfg.Port, result.Error)
		if firstCode == ExitAuth || firstCode == ExitUnavailable {
			break
		}
	}
	if err := writeResult(c, map[string]any{"ok": firstCode == 0, "results": results, "exit_code": firstCode}, strings.TrimSpace(out.String())); err != nil {
		return err
	}
	if firstCode != 0 {
		return &ExitError{Code: firstCode, Err: errResultWritten}
	}
	return nil
}

var errResultWritten = errors.New("批量操作包含失败条目")
