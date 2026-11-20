package channels

import (
	"strings"
	"unicode/utf8"
)

// Server酱对标题/正文的长度上限
const (
	maxTitleRunes = 32        // 标题按字符计
	maxDespBytes  = 32 * 1024 // 正文按字节计
)

// clampTitle 按字符数截断标题
func clampTitle(title string) string {
	return truncateRunes(title, maxTitleRunes)
}

// truncateRunes 按字符数上限截断，结果恒为合法 UTF-8。
//
// 用 []rune 转换而不是 s[:n]：Go 的字符串切片按字节走，切中文会留下半个
// UTF-8 字符。ToValidUTF8 先净化，标题可能来自用户自配的订阅模板。
func truncateRunes(s string, max int) string {
	s = strings.ToValidUTF8(s, string(utf8.RuneError))
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

// truncateBytes 按字节上限截断，回退到最近的字符边界，绝不切断多字节字符。
//
// 正文按字节计是接口实际口径，只能按字节切；但 s[:max] 合法当且仅当 s[max]
// 不是后续字节（后续字节 top two bits 恒为 10），据此往回退即是最大合法切点。
func truncateBytes(s string, max int) string {
	s = strings.ToValidUTF8(s, string(utf8.RuneError))
	if len(s) <= max {
		return s
	}
	for max > 0 && !utf8.RuneStart(s[max]) {
		max--
	}
	return s[:max]
}
