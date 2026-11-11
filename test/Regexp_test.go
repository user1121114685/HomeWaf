package test

import (
	"regexp"
	"testing"
)

// 特例：找出中英文"（）()"括号中间的字符，例如：华南地区（广州） -> 广州
func findDestStr(src string) string {
	compileRegex := regexp.MustCompile("（(.*?)）") // 中文括号，例如：华南地区（广州） -> 广州
	matchArr := compileRegex.FindStringSubmatch(src)

	if len(matchArr) == 0 {
		compileRegex := regexp.MustCompile("\\((.*?)\\)") // 兼容英文括号并取消括号的转义，例如：华南地区 (广州) -> 广州。
		matchArr = compileRegex.FindStringSubmatch(src)
	}
	// fmt.Println("提取字符串内容：", matchArr[len(matchArr)-1])

	if len(matchArr) > 0 {
		return matchArr[len(matchArr)-1]
	}
	return ""
}
func TestRegexp(t *testing.T) {
	str := "好喝的蜜桃乌龙茶，哈哈哈"
	compileRegex := regexp.MustCompile("json(.*?)，哈哈哈") // 正则表达式的分组，以括号()表示，每一对括号就是我们匹配到的一个文本，可以把他们提取出来。
	matchArr := compileRegex.FindStringSubmatch(str)    // FindStringSubmatch 方法是提取出匹配的字符串，然后通过[]string返回。我们可以看到，第1个匹配到的是这个字符串本身，从第2个开始，才是我们想要的字符串。
	if len(matchArr) != 0 {
		t.Fatal("不匹配的正则不应返回捕获组")
	}
	if got := findDestStr("华南地区（广州）"); got != "广州" {
		t.Fatalf("中文括号提取失败：%q", got)
	}
	if got := findDestStr("South China (Guangzhou)"); got != "Guangzhou" {
		t.Fatalf("英文括号提取失败：%q", got)
	}

}
