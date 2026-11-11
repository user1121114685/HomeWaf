package wafpipeline

import (
	lib "HomeWaf/libinjection-go"
	"HomeWaf/wafenginecore/wafhttpcore"
	"regexp"
	"strings"
)

var sqlComments = regexp.MustCompile(`/\*[^*]*(?:\*+[^*/][^*]*)*\*+/`)
var sqlContext = regexp.MustCompile(`(?i)(['"0-9)]\s*(?:or|and)\b|\bunion\s+(?:all\s+)?select\b|;\s*(?:select|drop|insert|update|delete|exec)\b|['"]\s*(?:--|#)|\b(?:select|delete|update|insert)\b.{0,500}\b(?:from|into|where)\b|\b(?:sleep|benchmark|pg_sleep)\s*\()`)
var xssExec = regexp.MustCompile(`(?i)<[a-z!/]|on\w+\s*=|javascript:|srcdoc\s*=|data:text/html`)

func SQLDetector() Detector {
	return valueDetector{"sqli", func(value string) bool {
		if wafhttpcore.HasDecodedHighRiskSQLKeyword(value) {
			return true
		}
		folded, compact := value, value
		if strings.Contains(value, "/*") {
			folded = sqlComments.ReplaceAllString(value, " ")
			compact = sqlComments.ReplaceAllString(value, "")
		}
		return sqlContext.MatchString(folded) && lib.IsSQLiNotReturnPrint(folded) || compact != folded && sqlContext.MatchString(compact) && lib.IsSQLiNotReturnPrint(compact)
	}}
}
func XSSDetector() Detector {
	return valueDetector{"xss", func(value string) bool {
		// The existing JS/unicode decoder is shared with the old request engine.
		value = wafhttpcore.NormalizeDecodedForXSSDetection(value)
		return xssExec.MatchString(value) && lib.IsXSSSingleValue(strings.TrimSpace(value))
	}}
}
