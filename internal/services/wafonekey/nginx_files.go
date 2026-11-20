package wafonekey

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"HomeWaf/internal/instance/global"
)

const (
	maxScanFileSize = 2 << 20
	defaultNginxDir = "/www/server/panel/vhost/nginx"
)

func openNginxDir(requested string) (*os.Root, error) {
	trusted := global.CurrentSettings().NginxConfigRoot
	if trusted == "" {
		trusted = defaultNginxDir
	} else if !filepath.IsAbs(trusted) {
		return nil, errors.New("security.nginx_config_root 必须是绝对目录")
	}
	base, err := filepath.Abs(trusted)
	if err != nil {
		return nil, err
	}
	if requested == "" {
		requested = base
	}
	requested, err = filepath.Abs(requested)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(base, requested)
	if err != nil || !filepath.IsLocal(rel) {
		return nil, errors.New("目录不在允许范围内；自定义宝塔安装路径请在 config.yml 配置 security.nginx_config_root")
	}
	root, err := os.OpenRoot(base)
	if err != nil || rel == "." {
		return root, err
	}
	defer root.Close()
	return root.OpenRoot(rel)
}

func readNginxConfig(root *os.Root, name string) ([]byte, os.FileMode, error) {
	if !filepath.IsLocal(name) || name != filepath.Base(name) || !strings.EqualFold(filepath.Ext(name), ".conf") {
		return nil, 0, errors.New("无效的 Nginx 配置文件名")
	}
	info, err := root.Lstat(name)
	if err != nil {
		return nil, 0, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxScanFileSize {
		return nil, 0, errors.New("配置文件必须是范围内的普通文件，大小不超过 2 MiB")
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, 0, errors.New("配置文件已变化，请重试")
	}
	data, err := io.ReadAll(io.LimitReader(f, maxScanFileSize+1))
	if len(data) > maxScanFileSize {
		return nil, 0, fmt.Errorf("配置文件超过 %d 字节", maxScanFileSize)
	}
	return data, opened.Mode().Perm(), err
}

// parseNginxContent 从配置文本中提取每个 server{} 块的候选
