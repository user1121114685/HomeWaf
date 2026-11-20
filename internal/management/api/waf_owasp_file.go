package api

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Read through a rooted handle so absolute paths and escaping symlinks cannot
// turn the read-only rule browser into an arbitrary server-file reader.
func readOwaspRuleFile(directory, name string) ([]byte, error) {
	if !filepath.IsLocal(name) || strings.ToLower(filepath.Ext(name)) != ".conf" {
		return nil, fmt.Errorf("只能查看规则目录内的 .conf 文件")
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	f, err := root.Open(name)
	if err != nil {
		return nil, fmt.Errorf("文件不存在或不可访问")
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("不是可读取的规则文件")
	}
	const limit = 2 << 20
	content, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if len(content) > limit {
		return nil, fmt.Errorf("文件超过 2 MiB 在线预览上限，请在服务器查看")
	}
	return content, nil
}
