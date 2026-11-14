package wafpipeline

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Zero-valued new fields preserve compatibility with existing saved policies.
func effectiveHoneypot(cfg HoneypotConfig) HoneypotConfig {
	if cfg.DropWindowSeconds == 0 {
		cfg.DropWindowSeconds = 600
	}
	if cfg.DropSeconds == 0 {
		cfg.DropSeconds = 10800
	}
	if cfg.DistributedDrop.WindowSeconds == 0 {
		cfg.DistributedDrop.WindowSeconds = 600
	}
	if cfg.DistributedDrop.DistinctIPs == 0 {
		cfg.DistributedDrop.DistinctIPs = 10
	}
	if cfg.DistributedDrop.Seconds == 0 {
		cfg.DistributedDrop.Seconds = 10800
	}
	if cfg.ArchiveMaxBytes == 0 {
		cfg.ArchiveMaxBytes = 16 << 10
	}
	return cfg
}

func validateHoneypotControls(cfg HoneypotConfig) error {
	cfg = effectiveHoneypot(cfg)
	d := cfg.DistributedDrop
	if cfg.DropAfterHits < 0 || cfg.DropAfterHits > 100000 || cfg.DropWindowSeconds < 1 || cfg.DropWindowSeconds > 86400 || cfg.DropSeconds < 1 || cfg.DropSeconds > 7*86400 || d.WindowSeconds < 1 || d.WindowSeconds > 86400 || d.DistinctIPs < 2 || d.DistinctIPs > 1000 || d.Seconds < 1 || d.Seconds > 7*86400 || cfg.ArchiveMaxBytes < 1024 || cfg.ArchiveMaxBytes > 64<<10 || len(cfg.Lures) > 32 {
		return fmt.Errorf("invalid honeypot drop or archive limits")
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
func selectHoneypotLure(s *Snapshot, c *RequestContext) HoneypotLure {
	lures := s.Config.Policy.Honeypot.Lures
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
