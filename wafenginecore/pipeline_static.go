package wafenginecore

import (
	"HomeWaf/wafpipeline"
	"errors"
	"net/http"
	"os"
	"path"
	"strings"
)

// Reuse the existing static-site security policy after the port pipeline's
// access, IP and authentication checks. os.Root keeps file opens confined even
// across symlinks; ServeContent retains Range and streaming without buffering.
func PipelineStaticHandler(business wafpipeline.StaticBusiness) (http.Handler, error) {
	config := business.Settings
	if config.StaticSitePath == "" {
		return nil, errors.New("静态站点根目录不能为空")
	}
	engine := &WafEngine{}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			http.Error(w, "Method not allowed", 405)
			return
		}
		if len(r.URL.Path) > 1024 {
			http.Error(w, "URI too long", 414)
			return
		}
		relative := strings.TrimPrefix(r.URL.Path, config.StaticSitePrefix)
		if engine.containsEncodedThreats(relative) || engine.containsPathTraversal(relative) || engine.isSensitivePath(relative, config) || engine.isSensitiveFile(relative, config) {
			http.Error(w, "Forbidden", 403)
			return
		}
		relative = strings.TrimPrefix(path.Clean("/"+relative), "/")
		if relative == "" || relative == "." {
			relative = "index.html"
		}
		root, err := os.OpenRoot(config.StaticSitePath)
		if err != nil {
			http.Error(w, "Static root unavailable", 503)
			return
		}
		defer root.Close()
		file, err := root.Open(relative)
		if err != nil && os.IsNotExist(err) && business.SPAFallback {
			relative = "index.html"
			file, err = root.Open(relative)
		}
		if err != nil {
			if os.IsNotExist(err) {
				http.NotFound(w, r)
			} else {
				http.Error(w, "Forbidden", 403)
			}
			return
		}
		info, err := file.Stat()
		if err != nil {
			file.Close()
			http.Error(w, "Forbidden", 403)
			return
		}
		if info.IsDir() {
			file.Close()
			relative = path.Join(relative, "index.html")
			file, err = root.Open(relative)
			if err != nil && business.SPAFallback {
				relative = "index.html"
				file, err = root.Open(relative)
			}
			if err != nil {
				http.NotFound(w, r)
				return
			}
			info, err = file.Stat()
		}
		defer file.Close()
		if err != nil || info.IsDir() || !engine.isAllowedFileType(relative, config) {
			http.Error(w, "Forbidden", 403)
			return
		}
		engine.setSecurityHeaders(w, config)
		http.ServeContent(w, r, relative, info.ModTime(), file)
	}), nil
}
