package zlog

import (
	"strings"
	"testing"
)

func TestSingleLineKeepsOneLogRecord(t *testing.T) {
	const input = "request path\nsecond line\rtrace\x1b"
	line := SingleLine(input)
	if strings.ContainsAny(line, "\r\n\x1b") || !strings.Contains(line, `\n`) || SingleLine("服务正常") != "服务正常" {
		t.Fatalf("unexpected log rendering: %q", line)
	}
}
