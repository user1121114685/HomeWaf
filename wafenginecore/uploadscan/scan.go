// Shared upload parsers and signatures extracted from the existing WAF engine.
package uploadscan

import (
	"HomeWaf/model"
	"bytes"
	"mime/multipart"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// PHP 脚本标记出现时才检查的危险函数（小写）
var uploadPHPDangerFuncs = []string{
	"eval(", "assert(", "system(", "passthru(", "shell_exec(", "popen(", "proc_open(",
	"base64_decode(", "gzinflate(", "str_rot13(", "create_function(", "call_user_func(",
	"$_post", "$_get", "$_request", "preg_replace(", "array_map(",
}

// ASP/JSP 脚本标记出现时才检查的危险调用（小写）
var uploadScriptDangerFuncs = []string{
	"runtime.getruntime", "processbuilder", ".exec(", "wscript.shell",
	"server.createobject", "eval request", "execute(", "eval(",
}

// 高可信正则：PHP 一句话木马（子串预过滤门控后才跑）
var reUploadPHPOneLiner = regexp.MustCompile(`(?i)(eval|assert)\s*\(\s*\$_(post|get|request|server|cookie)`)

// 图片扩展名（用于“声称图片却是脚本”判断；不含 svg，svg 本身是 xml 易误报）
var uploadImageExts = map[string]bool{
	"jpg": true, "jpeg": true, "png": true, "gif": true, "bmp": true, "webp": true, "ico": true, "tiff": true,
}

// ExtractExts 提取文件名的所有扩展名段（去空字节、取 basename、小写、按 . 拆），用于双扩展名/空字节绕过检测
func ExtractExts(filename string) []string {
	name := filename
	if i := strings.IndexByte(name, 0x00); i >= 0 { // 空字节截断 x.php\x00.jpg
		name = name[:i]
	}
	if idx := strings.LastIndexAny(name, "/\\"); idx >= 0 {
		name = name[idx+1:]
	}
	name = strings.ToLower(strings.TrimSpace(name))
	if i := strings.IndexByte(name, ':'); i >= 0 && i != 1 {
		name = name[:i]
	}
	parts := strings.Split(name, ".")
	if len(parts) <= 1 {
		return nil
	}
	exts := make([]string, 0, len(parts)-1)
	for _, p := range parts[1:] {
		p = strings.TrimRight(strings.TrimSpace(p), ". ")
		if p != "" {
			exts = append(exts, p)
		}
	}
	return exts
}

// DangerousExt 文件名任一扩展名段命中黑名单即危险（防 shell.php.jpg 双扩展名）
var defaultDangerousExts = func() map[string]bool {
	set := map[string]bool{}
	for _, e := range strings.Split(model.DefaultUploadExtBlacklist, ",") {
		set[e] = true
	}
	return set
}()

func DangerousExt(filename, blacklist string) (bool, string) {
	bl := blacklist
	if strings.TrimSpace(bl) == "" {
		bl = model.DefaultUploadExtBlacklist
	}
	set := defaultDangerousExts
	if bl != model.DefaultUploadExtBlacklist {
		set = map[string]bool{}
		for _, e := range strings.Split(bl, ",") {
			e = strings.ToLower(strings.TrimSpace(e))
			if e != "" {
				set[e] = true
			}
		}
	}
	for _, ext := range ExtractExts(filename) {
		if set[ext] {
			return true, ext
		}
	}
	return false, ""
}

// reUploadCDFilename 抓 Content-Disposition 里出现的每一个 filename / filename* / filename*N* 参数原值
var reUploadCDFilename = regexp.MustCompile(`(?i)(?:^|;)\s*filename\s*(\*\s*\d*\s*\*?)?\s*=\s*("(?:[^"\\]|\\.)*"|[^;]*)`)

// unquoteCDValue 去引号并还原 \" 转义
func unquoteCDValue(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		inner := v[1 : len(v)-1]
		var b strings.Builder
		for i := 0; i < len(inner); i++ {
			if inner[i] == '\\' && i+1 < len(inner) {
				i++
			}
			b.WriteByte(inner[i])
		}
		return b.String()
	}
	return v
}

// stripRFC2231Charset 去掉 charset'lang' 前缀（无论该字符集 Go 是否支持——后端往往照收）
func stripRFC2231Charset(v string) string {
	if i := strings.Index(v, "'"); i >= 0 {
		if j := strings.Index(v[i+1:], "'"); j >= 0 {
			return v[i+1+j+1:]
		}
	}
	return v
}

// percentDecodeLoose 宽松百分号解码，非法序列原样保留
func percentDecodeLoose(s string) string {
	if !strings.Contains(s, "%") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			if h, err := strconv.ParseUint(s[i+1:i+3], 16, 8); err == nil {
				b.WriteByte(byte(h))
				i += 2
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// PartFileNames 收集该 part 所有“可能被后端解析成文件名”的候选。
// Go 的 part.FileName() 会丢弃不支持字符集的 filename*、并在参数重复时整体报错返回空，
// 而后端(Werkzeug/PHP 等)照样能解析出 shell.php —— 只信任单一解析结果即形成绕过。
func PartFileNames(part *multipart.Part) []string {
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	addWithDecoded := func(s string) {
		add(s)
		if d := percentDecodeLoose(s); d != s {
			add(d)
		}
	}
	add(part.FileName())
	var segs []string
	// Values 而非 Get：头部重复出现时 Go 只取第一条，后端可能取最后一条
	for _, cd := range part.Header.Values("Content-Disposition") {
		for _, m := range reUploadCDFilename.FindAllStringSubmatch(cd, -1) {
			star := strings.TrimSpace(m[1])
			val := unquoteCDValue(m[2])
			if star != "" { // 扩展形式 filename* / filename*N*
				if strings.HasSuffix(star, "*") {
					val = stripRFC2231Charset(val)
				}
				segs = append(segs, val)
			}
			addWithDecoded(val)
		}
	}
	if len(segs) > 1 { // RFC2231 分段拼接：filename*0*=sh; filename*1*=ell.php
		addWithDecoded(strings.Join(segs, ""))
	}
	return out
}

// OverSize 文件大小是否超过上限
func OverSize(size, maxKB int) bool {
	return maxKB > 0 && size > maxKB*1024
}

// MatchPathPrefix 路径是否命中任一前缀（换行分隔）
func MatchPathPrefix(path, lines string) bool {
	for _, line := range strings.Split(lines, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && strings.HasPrefix(path, line) {
			return true
		}
	}
	return false
}

// HasImageExt 文件名是否是图片扩展名（不含 svg）
func HasImageExt(filename string) bool {
	exts := ExtractExts(filename)
	if len(exts) == 0 {
		return false
	}
	return uploadImageExts[exts[len(exts)-1]]
}

// TypeMismatch 保守判断“声称图片/媒体但真实内容是脚本/HTML”
func TypeMismatch(filename, declaredCT string, head []byte) bool {
	claimImage := strings.HasPrefix(strings.ToLower(declaredCT), "image/") || HasImageExt(filename)
	if !claimImage || len(head) == 0 {
		return false
	}
	real := strings.ToLower(http.DetectContentType(head))
	if strings.HasPrefix(real, "image/") {
		return false // 真的是图片，放行
	}
	// 声称图片但真实是 html/text/xml，或内容含脚本标记 → 不符
	if strings.HasPrefix(real, "text/") || strings.Contains(real, "html") || strings.Contains(real, "xml") {
		return true
	}
	lower := bytes.ToLower(head)
	if bytes.Contains(lower, []byte("<?php")) || bytes.Contains(lower, []byte("<?=")) ||
		bytes.Contains(lower, []byte("<%")) || bytes.Contains(lower, []byte("<script")) {
		return true
	}
	return false
}

// Webshell 只扫传入的头部字节：脚本标记 + 危险函数组合判定（低误报），命中返回特征名
func Webshell(head []byte) (bool, string) {
	if len(head) == 0 {
		return false, ""
	}
	lower := bytes.ToLower(head)
	// 1) 高可信：PHP 一句话木马
	if (bytes.Contains(lower, []byte("<?")) || bytes.Contains(lower, []byte("$_"))) && reUploadPHPOneLiner.Match(lower) {
		return true, "php-eval-superglobal"
	}
	// 2) PHP 脚本标记 + 危险函数
	if bytes.Contains(lower, []byte("<?php")) || bytes.Contains(lower, []byte("<?=")) {
		for _, f := range uploadPHPDangerFuncs {
			if bytes.Contains(lower, []byte(f)) {
				return true, "php:" + f
			}
		}
	}
	// 3) ASP/JSP 脚本标记 + 危险调用
	if bytes.Contains(lower, []byte("<%")) {
		for _, f := range uploadScriptDangerFuncs {
			if bytes.Contains(lower, []byte(f)) {
				return true, "script:" + f
			}
		}
	}
	return false, ""
}
