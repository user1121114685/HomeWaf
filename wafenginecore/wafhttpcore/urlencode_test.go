// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package wafhttpcore

import (
	"HomeWaf/libinjection-go"
	"net/url"
	"testing"
)

func TestWafHttpCoreUrlEncode(t *testing.T) {
	srcUrl := "1=1sion%28%29%2523"
	t.Log(srcUrl)
	en := WafHttpCoreUrlEncode(srcUrl, 100)
	t.Log(en)
	en2, ok := url.QueryUnescape(srcUrl)
	if ok != nil {
		t.Error(ok)
	}
	t.Log(en2)
	srcCheckResult := libinjection.IsSQLiNotReturnPrint(srcUrl)
	t.Log(srcCheckResult)
	targetCheckResult := libinjection.IsSQLiNotReturnPrint(en)
	t.Log(targetCheckResult)
	targetCheckResult = libinjection.IsXSS(en)
	t.Log(targetCheckResult)
}
