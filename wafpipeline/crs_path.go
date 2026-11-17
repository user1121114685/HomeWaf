package wafpipeline

import (
	"path"
	"strings"

	"github.com/corazawaf/coraza/v3/experimental/plugins"
)

// CRS patterns describe HTTP paths, not paths on the WAF host. Coraza 3.8.1
// uses filepath.Clean, which changes separators on Windows and prevents its
// forward-slash phrase lists from matching. Keep normalization host-independent.
func normalizeCRSPath(value string) (string, bool, error) {
	if value == "" {
		return value, false, nil
	}
	clean := path.Clean(value)
	if clean == "." {
		clean = ""
	} else if strings.HasSuffix(value, "/") && !strings.HasSuffix(clean, "/") {
		clean += "/"
	}
	return clean, clean != value, nil
}

func normalizeCRSWindowsPath(value string) (string, bool, error) {
	clean, _, _ := normalizeCRSPath(strings.ReplaceAll(value, `\`, "/"))
	// Preserve drive prefixes, but resolve NTFS alternate data streams against
	// the final filename. Keep the existing Windows trailing-dot/space defense.
	drive := ""
	if len(clean) >= 2 && clean[1] == ':' && ((clean[0] >= 'A' && clean[0] <= 'Z') || (clean[0] >= 'a' && clean[0] <= 'z')) {
		drive, clean = clean[:2], clean[2:]
	}
	trailing := strings.HasSuffix(clean, "/")
	if trailing {
		clean = strings.TrimSuffix(clean, "/")
	}
	last := strings.LastIndexByte(clean, '/') + 1
	if colon := strings.IndexByte(clean[last:], ':'); colon >= 0 {
		clean = clean[:last+colon]
	}
	parts := strings.Split(clean, "/")
	kept := parts[:0]
	for _, part := range parts {
		if strings.Trim(part, ".") != "" {
			part = strings.TrimRight(part, ". ")
			if part == "" {
				continue
			}
		}
		kept = append(kept, part)
	}
	clean = drive + strings.Join(kept, "/")
	if trailing {
		clean += "/"
	}
	return clean, clean != value, nil
}

func init() {
	plugins.RegisterTransformation("normalizePath", normalizeCRSPath)
	plugins.RegisterTransformation("normalisePath", normalizeCRSPath)
	plugins.RegisterTransformation("normalizePathWin", normalizeCRSWindowsPath)
	plugins.RegisterTransformation("normalisePathWin", normalizeCRSWindowsPath)
}
