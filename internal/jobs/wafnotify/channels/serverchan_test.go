package channels

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// 复现渠道测试的标题：[<12位容器ID>] HomeWaf 测试通知
// Server酱限定 32 个字符，中文标题需要在完整字符边界截断。
func TestClampTitleKeepsWholeRunes(t *testing.T) {
	cases := []string{
		"[28c42919efe3] HomeWaf 测试通知",
		"【测试】安全规则触发通知",
		"[a-very-long-instance-name-here] 服务器入侵防火墙告警",
		"纯中文标题用来验证截断边界是否落在字符边界上并且不会损坏编码",
	}

	for _, in := range cases {
		got := clampTitle(in)
		if !utf8.ValidString(got) {
			t.Fatalf("截断后不是合法 UTF-8: %q", got)
		}
		if utf8.RuneCountInString(got) > maxTitleRunes {
			t.Fatalf("截断后仍超长: %d 字符, %q", utf8.RuneCountInString(got), got)
		}
		if in == got {
			continue
		}
		// 截断结果必须是原串的前缀，且不能切碎字符
		if !strings.HasPrefix(in, got) {
			t.Fatalf("截断结果不是原串前缀: in=%q got=%q", in, got)
		}
		if next := in[len(got):]; next != "" && !utf8.RuneStart(next[0]) {
			t.Fatalf("切在了字符中间: got=%q 剩余=%q", got, next)
		}
	}
}

func TestClampTitleShortInputUntouched(t *testing.T) {
	in := "HomeWaf 测试通知"
	if got := clampTitle(in); got != in {
		t.Fatalf("未超长时不应改动: got=%q want=%q", got, in)
	}
}

func TestTruncateBytesStaysOnRuneBoundary(t *testing.T) {
	// 构造一个正好切在汉字中间的字节上限
	in := strings.Repeat("测", 100)
	for max := 1; max <= 20; max++ {
		got := truncateBytes(in, max)
		if !utf8.ValidString(got) {
			t.Fatalf("max=%d 截出了非法 UTF-8: %q", max, got)
		}
		if len(got) > max {
			t.Fatalf("max=%d 结果超长: %d", max, len(got))
		}
	}
}

func TestTruncateSanitizesInvalidUTF8(t *testing.T) {
	bad := "abc\xe7" // 半个 UTF-8 字符（Server酱报错里那个字节）
	for _, got := range []string{clampTitle(bad), truncateBytes(bad, maxDespBytes)} {
		if !utf8.ValidString(got) {
			t.Fatalf("未净化非法字节: %q", got)
		}
	}
}

func TestClampTitleAtMultibyteBoundary(t *testing.T) {
	prefix := strings.Repeat("a", maxTitleRunes-1)
	got := clampTitle(prefix + "测试")
	if got != prefix+"测" {
		t.Fatalf("truncated title %q", got)
	}
}
