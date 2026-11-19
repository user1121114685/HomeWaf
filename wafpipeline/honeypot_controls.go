package wafpipeline

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strings"
	"unicode/utf8"
)

func DefaultHoneypotConfig() HoneypotConfig {
	return HoneypotConfig{MaxConcurrent: 32, MaxDelayMillis: 250, ArchiveMaxBytes: 16 << 10}
}

func ValidateHoneypotConfig(cfg HoneypotConfig) error {
	if cfg.MaxConcurrent < 1 || cfg.MaxConcurrent > 32 || cfg.MaxDelayMillis < 0 || cfg.MaxDelayMillis > 2000 || cfg.ArchiveMaxBytes < 1024 || cfg.ArchiveMaxBytes > 64<<10 || len(cfg.TrapPaths) > 128 || len(cfg.Lures) > 32 {
		return fmt.Errorf("invalid honeypot resource limits")
	}
	for _, path := range cfg.TrapPaths {
		if !strings.HasPrefix(path, "/") || len(path) > 1024 || NormalizePath(path) != path {
			return fmt.Errorf("honeypot trap paths must be canonical absolute paths")
		}
	}
	ids, total := map[string]bool{}, 0
	for _, lure := range cfg.Lures {
		total += len(lure.Title) + len(lure.Message)
		if strings.TrimSpace(lure.ID) == "" || len(lure.ID) > 64 || ids[lure.ID] || !utf8.ValidString(lure.ID+lure.Title+lure.Message) || strings.TrimSpace(lure.Title) == "" || len(lure.Title) > 128 || len(lure.Message) > 4096 || lure.Weight < 1 || lure.Weight > 10000 || total > 32<<10 {
			return fmt.Errorf("invalid honeypot lure or text quota")
		}
		ids[lure.ID] = true
	}
	return nil
}

// The same source sees a stable weighted choice without trusting a cookie or
// issuing any client-side script. This server never uses display text as
// instructions to a model or tool.
func selectHoneypotLure(s *Snapshot, c *RequestContext, lures []HoneypotLure) HoneypotLure {
	total := 0
	for _, lure := range lures {
		if lure.Weight > 0 {
			total += lure.Weight
		}
	}
	if total == 0 {
		return HoneypotLure{ID: "default", Title: "管理控制台", Message: "请输入管理账号。", Weight: 1}
	}
	mac := hmac.New(sha256.New, []byte(s.Secrets.GateKey))
	_, _ = fmt.Fprintf(mac, "HomeWaf honeypot lure v1\n%s\n%s", c.PortID, c.ClientIP.Unmap())
	choice := int(binary.BigEndian.Uint64(mac.Sum(nil)[:8]) % uint64(total))
	for _, lure := range lures {
		if lure.Weight <= 0 {
			continue
		}
		if choice < lure.Weight {
			return lure
		}
		choice -= lure.Weight
	}
	return lures[0]
}
