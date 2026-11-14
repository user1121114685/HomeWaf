package wafpipeline

import (
	"HomeWaf/internal/logsanitize"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/http"
	"time"
)

// Encoded samples are inert log data. No executable files, extraction paths,
// HTML previews, shell processes or model invocations are created from them.
type HoneypotArchive struct {
	PrefixSHA256 string `json:"prefix_sha256"`
	SampleBytes  int    `json:"sample_bytes"`
	Truncated    bool   `json:"truncated"`
	ContentType  string `json:"content_type"`
	BodyBase64   string `json:"body_base64,omitempty"`
	Omitted      bool   `json:"omitted"`
}

func readHoneypotBody(w http.ResponseWriter, c *RequestContext, cfg HoneypotConfig) []byte {
	limit := 16 << 10
	if cfg.ArchiveBody {
		limit = cfg.ArchiveMaxBytes
	}
	controller := http.NewResponseController(w)
	_ = controller.SetReadDeadline(time.Now().Add(5 * time.Second))
	defer func() { _ = controller.SetReadDeadline(time.Time{}) }()
	raw, err := io.ReadAll(http.MaxBytesReader(w, c.Request.Body, int64(limit)+1))
	truncated := err != nil || len(raw) > limit
	if len(raw) > limit {
		raw = raw[:limit]
	}
	// The isolated login parser may inspect this bounded copy only.
	c.Request.Body = io.NopCloser(bytes.NewReader(raw))
	if cfg.ArchiveBody {
		digest := sha256.Sum256(raw)
		archive := &HoneypotArchive{PrefixSHA256: hex.EncodeToString(digest[:]), SampleBytes: len(raw), Truncated: truncated, ContentType: bound(c.Request.Header.Get("Content-Type"), 128), Omitted: true}
		if !truncated {
			if sample := logsanitize.Body(string(raw)); sample != "" && sample != logsanitize.Marker && len(sample) <= limit {
				archive.BodyBase64 = base64.StdEncoding.EncodeToString([]byte(sample))
				archive.Omitted = false
			}
		}
		if interaction, ok := c.Values["honeypot_interaction"].(*HoneypotInteraction); ok {
			interaction.Archive = archive
		}
	}
	return raw
}
