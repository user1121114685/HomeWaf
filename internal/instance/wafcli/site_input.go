// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package wafcli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"HomeWaf/internal/protection/wafpipeline"
)

const maxInputBytes = 10 << 20

func readSiteInput(path string, stdin io.Reader) ([]json.RawMessage, error) {
	var r io.Reader = stdin
	if path != "-" {
		f, err := os.Open(filepath.Clean(path))
		if err != nil {
			return nil, fail(ExitUsage, errors.New("无法读取站点配置文件"))
		}
		defer f.Close()
		r = f
	}
	b, err := io.ReadAll(io.LimitReader(r, maxInputBytes+1))
	if err != nil || len(b) > maxInputBytes {
		return nil, fail(ExitUsage, errors.New("读取配置失败或文件超过10MB"))
	}
	b = bytes.TrimSpace(b)
	var items []json.RawMessage
	if len(b) > 0 && b[0] == '{' {
		items = []json.RawMessage{b}
	} else {
		err = json.Unmarshal(b, &items)
	}
	if err != nil || len(items) == 0 {
		return nil, fail(ExitUsage, errors.New("配置必须是JSON对象或非空对象数组"))
	}
	seen := map[uint16]bool{}
	for i, item := range items {
		var cfg wafpipeline.PortConfig
		if err := wafpipeline.StrictDecode(item, &cfg); err != nil || cfg.Port == 0 {
			return nil, fail(ExitUsage, fmt.Errorf("第%d项：请提供包含有效端口号的服务配置并检查字段类型", i+1))
		}
		if seen[cfg.Port] {
			return nil, fail(ExitUsage, fmt.Errorf("第%d项：重复的端口 %d", i+1, cfg.Port))
		}
		seen[cfg.Port] = true
	}
	return items, nil
}

func sitePort(ref string) (uint16, error) {
	port, err := strconv.ParseUint(ref, 10, 16)
	if err != nil || port == 0 {
		return 0, fail(ExitUsage, errors.New("服务标识必须是1至65535之间的端口号"))
	}
	return uint16(port), nil
}
