package wafpipeline

import (
	"HomeWaf/model"
	"HomeWaf/wafdefenserce"
	"HomeWaf/wafenginecore/uploadscan"
	"HomeWaf/wafenginecore/wafhttpcore"
	"strings"
)

func LFIDetector() Detector {
	return valueDetector{"lfi", func(value string) bool {
		value = strings.ReplaceAll(strings.ReplaceAll(value, "..;", ".."), "\\", "/")
		lower := strings.ToLower(value)
		for _, scheme := range []string{"php://filter", "file://", "expect://"} {
			if strings.Contains(lower, scheme) {
				return true
			}
		}
		return wafhttpcore.HasDecodedTraversalOrLFI(value)
	}}
}
func RCEDetector() Detector {
	return valueDetector{"rce", func(value string) bool {
		// NAS documentation often contains ordinary command words. Shell syntax
		// must accompany the existing contextual OS-command signatures.
		if !strings.ContainsAny(value, ";|&\r\n`$") {
			return false
		}
		hit, _ := wafdefenserce.DetermineRCE(value)
		return hit
	}}
}

type uploadDetector struct{ options *model.UploadSecurityConfig }

func (uploadDetector) Name() string     { return "upload" }
func (uploadDetector) Category() string { return "upload" }
func UploadDetector() Detector          { return uploadDetector{} }
func (d uploadDetector) Inspect(v *NormalizedView) []Match {
	cfg := model.UploadSecurityConfig{IsEnable: 1, CheckExt: 1, CheckContent: 1, CheckMagic: 1}
	if d.options != nil {
		cfg = *d.options
		if cfg.IsEnable != 1 || !strings.HasPrefix(strings.ToLower(v.Request.Header.Get("Content-Type")), "multipart/") || uploadscan.MatchPathPrefix(v.Path, cfg.ExcludePaths) || cfg.IncludePaths != "" && !uploadscan.MatchPathPrefix(v.Path, cfg.IncludePaths) {
			return nil
		}
		if v.BodyTruncated && cfg.OverLimitAction == "block" {
			return []Match{{"upload", "multipart_file", "inspection_limit"}}
		}
	}
	for _, f := range v.Files {
		if v.expired() {
			break
		}
		if v.excluded(f.Name) {
			continue
		}
		names := f.Filenames
		if len(names) == 0 {
			names = []string{f.Filename}
		}
		for _, name := range names {
			start := strings.LastIndexAny(name, "/\\") + 1
			if colon := strings.IndexByte(name[start:], ':'); colon >= 0 && start+colon != 1 {
				name = name[:start+colon]
			}
			name = strings.TrimRight(name, " .")
			if bad, _ := uploadscan.DangerousExt(name, cfg.ExtBlacklist); cfg.CheckExt == 1 && bad {
				return []Match{{"upload", "multipart_file", "dangerous_extension"}}
			}
			if cfg.CheckMagic == 1 && uploadscan.TypeMismatch(name, f.ContentType, f.Prefix) {
				return []Match{{"upload", "multipart_file", "content_type_mismatch"}}
			}
		}
		if cfg.CheckSize == 1 && cfg.MaxSizeKB > 0 && uploadscan.OverSize(int(f.Size), cfg.MaxSizeKB) {
			return []Match{{"upload", "multipart_file", "file_size"}}
		}
		if hit, _ := uploadscan.Webshell(f.Prefix); cfg.CheckContent == 1 && hit {
			return []Match{{"upload", "multipart_file", "script_content"}}
		}
	}
	return nil
}
