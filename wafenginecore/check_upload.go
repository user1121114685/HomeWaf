// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package wafenginecore

import (
	"HomeWaf/innerbean"
	"HomeWaf/internal/bodyinspect"
	"HomeWaf/model"
	"HomeWaf/model/detection"
	"HomeWaf/model/wafenginmodel"
	"HomeWaf/wafenginecore/uploadscan"
	"bytes"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
)

const (
	uploadScanHeadBytes = 64 * 1024 // 每个文件只扫前 64KB 找 webshell 特征
	uploadBufferSlack   = 64 * 1024 // 读 body 时相对单文件上限的余量(表单字段+boundary)
)

func extractUploadExts(name string) []string { return uploadscan.ExtractExts(name) }
func matchDangerousExt(name, blacklist string) (bool, string) {
	return uploadscan.DangerousExt(name, blacklist)
}
func uploadPartFileNames(part *multipart.Part) []string { return uploadscan.PartFileNames(part) }
func overUploadSize(size, maxKB int) bool               { return uploadscan.OverSize(size, maxKB) }
func matchUploadPathPrefix(p, lines string) bool        { return uploadscan.MatchPathPrefix(p, lines) }
func hasUploadImageExt(name string) bool                { return uploadscan.HasImageExt(name) }
func isUploadTypeMismatch(name, ct string, head []byte) bool {
	return uploadscan.TypeMismatch(name, ct, head)
}
func scanWebshell(head []byte) (bool, string) { return uploadscan.Webshell(head) }

// uploadBlock 构造拦截结果并标记风险等级
func uploadBlock(weblog *innerbean.WebLog, title, content string) detection.Result {
	weblog.RISK_LEVEL = 3
	return detection.Result{IsBlock: true, Title: title, Content: content}
}

// CheckUpload 文件上传内容检测：对 multipart 上传做 扩展名/大小/类型不符/Webshell 四维检测。
// 关键：禁用 r.ParseMultipartForm（会清空 body 致后端丢包）；读整份 body 到内存解析副本并复位 r.Body。
func (waf *WafEngine) CheckUpload(r *http.Request, weblogbean *innerbean.WebLog, formValue url.Values,
	hostTarget *wafenginmodel.HostSafe, globalHostTarget *wafenginmodel.HostSafe) detection.Result {
	result := detection.Result{JumpGuardResult: false, IsBlock: false, Title: "", Content: ""}

	cfg := model.ParseUploadSecurityConfig(hostTarget.Host.UploadSecurityJSON)
	if cfg.IsEnable != 1 {
		return result
	}
	// 只对可能携带上传体的方法（其余零成本放行，不碰 body）
	m := strings.ToUpper(r.Method)
	if m != http.MethodPost && m != http.MethodPut && m != http.MethodPatch {
		return result
	}
	// 必须是 multipart/form-data 且能取到 boundary
	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		return result
	}
	boundary := params["boundary"]
	if boundary == "" || r.Body == nil {
		return result
	}
	// 路径门：IncludePaths 非空则只检测命中的；ExcludePaths 命中则跳过（优先）
	if strings.TrimSpace(cfg.IncludePaths) != "" && !matchUploadPathPrefix(r.URL.Path, cfg.IncludePaths) {
		return result
	}
	if matchUploadPathPrefix(r.URL.Path, cfg.ExcludePaths) {
		return result
	}

	// 读 body（限检测缓冲上限）并复位 r.Body，保证后端能收到完整上传
	limit := int64(cfg.MaxSizeKB)*1024 + uploadBufferSlack
	if limit > 16<<20 {
		limit = 16 << 20
	}
	raw, readErr := bodyinspect.ReadPrefix(r, limit)
	if readErr != nil {
		return uploadBlock(weblogbean, "文件上传检测-读取异常", "上传体读取失败，已拦截")
	}

	// 请求体超过可检测上限：fail-closed 默认拦，避免“填大绕过内容检测”
	if int64(len(raw)) > limit {
		if strings.EqualFold(cfg.OverLimitAction, "pass") {
			return result
		}
		return uploadBlock(weblogbean, "文件上传检测-上传体过大", "上传体超过可检测上限，已拦截")
	}

	mr := multipart.NewReader(bytes.NewReader(raw), boundary)
	for {
		// NextRawPart：不做 quoted-printable 解码，扫到的字节与后端收到的一致
		part, e := mr.NextRawPart()
		if e != nil {
			if errors.Is(e, io.EOF) {
				break
			}
			// 报文畸形会让扫描提前终止而后端仍能解析出后续 part，fail-closed
			return uploadBlock(weblogbean, "文件上传检测-报文异常", "multipart报文解析异常，已拦截")
		}
		fileNames := uploadPartFileNames(part)
		if len(fileNames) == 0 { // 非文件字段跳过（文本字段由 SQLi/XSS 检测覆盖）
			part.Close()
			continue
		}
		fn := part.FileName()
		if fn == "" { // Go 解析不出但后端能解析出，取候选做后续内容判定
			fn = fileNames[0]
		}
		declaredCT := part.Header.Get("Content-Type")
		fileContent, _ := io.ReadAll(io.LimitReader(part, limit+1))
		part.Close()

		// 便宜→贵，命中即短路
		if cfg.CheckExt == 1 {
			for _, name := range fileNames { // 逐个候选校验，防解析差异绕过
				if bad, ext := matchDangerousExt(name, cfg.ExtBlacklist); bad {
					return uploadBlock(weblogbean, "文件上传检测-危险扩展名", "检测到危险文件扩展名(."+ext+")，已拦截")
				}
			}
		}
		if cfg.CheckSize == 1 && overUploadSize(len(fileContent), cfg.MaxSizeKB) {
			return uploadBlock(weblogbean, "文件上传检测-文件过大", "上传文件超过大小上限，已拦截")
		}
		head := fileContent
		if len(head) > uploadScanHeadBytes {
			head = head[:uploadScanHeadBytes]
		}
		if cfg.CheckMagic == 1 && isUploadTypeMismatch(fn, declaredCT, head) {
			return uploadBlock(weblogbean, "文件上传检测-类型不符", "文件真实内容与声明类型不符，已拦截")
		}
		if cfg.CheckContent == 1 {
			if hit, sig := scanWebshell(head); hit {
				return uploadBlock(weblogbean, "文件上传检测-Webshell", "检测到Webshell特征("+sig+")，已拦截")
			}
		}
	}
	return result
}
