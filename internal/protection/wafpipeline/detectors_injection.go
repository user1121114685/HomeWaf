package wafpipeline

import (
	"regexp"
	"strings"

	"HomeWaf/internal/protection/wafenginecore/wafhttpcore"
	"github.com/corazawaf/libinjection-go"
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
		return matchesSQLFingerprint(folded) || compact != folded && matchesSQLFingerprint(compact)
	}}
}
func matchesSQLFingerprint(value string) bool {
	if !sqlContext.MatchString(value) {
		return false
	}
	matched, _ := libinjection.IsSQLi(value)
	return matched
}

func XSSDetector() Detector {
	return valueDetector{"xss", func(value string) bool {
		// The existing JS/unicode decoder is shared with the old request engine.
		value = wafhttpcore.NormalizeDecodedForXSSDetection(value)
		if !xssExec.MatchString(value) {
			return false
		}
		value = strings.TrimSpace(value)
		return containsXSSChars(value) && libinjection.IsXSS(value)
	}}
}

func containsXSSChars(s string) bool {
	for _, c := range s {
		switch c {
		case '<', '>', '"', '\'', '(', ')', '/', '`':
			return true
		}
	}
	return false
}
