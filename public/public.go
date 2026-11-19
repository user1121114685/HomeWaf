package public

import (
	"embed"
	"io/fs"
	"strings"
)

//go:embed all:dist
var dist embed.FS

//go:embed web_frontend.txt
var webFrontendStamp string

var Public, _ = fs.Sub(dist, "dist")

// WebFrontendVersion 返回内嵌管理端前端的版本。
// scripts/build.mjs 在打包时从 web/package.json 写入版本。
func WebFrontendVersion() string {
	for _, line := range strings.Split(webFrontendStamp, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "version="); ok {
			if v = strings.TrimSpace(v); v != "" {
				return v
			}
		}
	}
	return "未知"
}
