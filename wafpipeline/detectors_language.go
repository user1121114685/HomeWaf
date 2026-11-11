package wafpipeline

import (
	"bytes"
	"regexp"
	"strings"
)

var (
	lookupCase     = regexp.MustCompile(`(?i)\$\{(?:lower|upper):([a-z])\}`)
	lookupDefault  = regexp.MustCompile(`\$\{[^{}]{0,24}:-([^{}]{1,16})\}`)
	javaExpression = regexp.MustCompile(`(?i)(\$\{\s*(?:jndi|ctx)\s*:|Runtime\s*\.\s*getRuntime\s*\(\s*\)\s*\.\s*exec\s*\(|(?:java\.lang\.)?ProcessBuilder\s*\(|T\s*\(\s*java\.lang\.Runtime\s*\)|@java\.lang\.Runtime@|#_memberAccess|#context\s*\[\s*['"]xwork\.MethodAccessor|com\.sun\.rowset\.JdbcRowSetImpl)`)
	javaGadget     = regexp.MustCompile(`(?i)(org\.apache\.commons\.collections[34]?\..*(?:InvokerTransformer|ChainedTransformer)|com\.sun\.org\.apache\.xalan\.internal\.xsltc\.trax\.TemplatesImpl|org\.codehaus\.groovy\.runtime\.ConvertedClosure|org\.springframework\.beans\.factory\.ObjectFactory)`)
	phpSerialized  = regexp.MustCompile(`(?i)(?:^|[\s;{}])(?:[OC]:[0-9]{1,6}:"[^"\r\n]{1,128}":[0-9]{1,6}:\{|a:[0-9]{1,6}:\{)`)
	phpCode        = regexp.MustCompile(`(?i)(<\?(?:php\b|=)|php://input|data://[^\s]{0,80}(?:text/plain|base64)|\b(?:eval|assert|system|exec|passthru|shell_exec|preg_replace)\s*\(\s*(?:\$|['"]|base64_decode\s*\(|gzinflate\s*\(|str_rot13\s*\())`)
	aspCode        = regexp.MustCompile(`(?i)(<%\s*(?:=|execute\b|eval\b|response\b|server\b|dim\b)|\b(?:execute|eval)\s*\(\s*['"]|server\.createobject\s*\(\s*["'](?:wscript\.shell|adodb\.|scripting\.filesystemobject))`)
	templateCode   = regexp.MustCompile(`(?i)(\{\{\s*\d+\s*[+*/-]\s*\d+\s*\}\}|\$\{\s*\d+\s*[+*/-]\s*\d+\s*\}|#\{\s*\d+\s*[+*/-]\s*\d+\s*\}|\{\{[^\r\n]{0,512}(?:__class__|__globals__|__mro__|__subclasses__|config\b)|<#assign\s+[^>]{0,512}(?:Execute|\?new|Runtime)|#set\s*\(\s*\$[^\r\n]{0,512}(?:getRuntime|forName|exec\s*\()|\$\{[^\r\n]{0,512}T\s*\(\s*java\.lang\.Runtime)`)
)

func foldLookups(s string) string {
	for i := 0; i < 4; i++ {
		// Both replacements require this literal marker. Ordinary request fields
		// need no regexp replacement or temporary string allocation.
		if !strings.Contains(s, "${") {
			break
		}
		next := lookupCase.ReplaceAllString(s, "$1")
		next = lookupDefault.ReplaceAllString(next, "$1")
		if next == s {
			break
		}
		s = next
	}
	return s
}
func JavaInjectionDetector() Detector {
	return valueDetector{"java_inject", func(s string) bool { return javaExpression.MatchString(foldLookups(s)) }}
}
func PHPDeserializationDetector() Detector {
	return valueDetector{"php_deser", func(s string) bool {
		return phpSerialized.MatchString(s) || strings.Contains(strings.ToLower(s), "phar://")
	}}
}
func PHPInjectionDetector() Detector { return valueDetector{"php_inject", phpCode.MatchString} }
func ASPInjectionDetector() Detector { return valueDetector{"asp_inject", aspCode.MatchString} }
func SSTIDetector() Detector {
	return valueDetector{"ssti", func(s string) bool {
		// Every alternative requires a literal '{' or '#', including #set and
		// <#assign. Check only necessary punctuation, never a guessed keyword:
		// Unicode case folding and all existing regexp semantics stay intact.
		return strings.ContainsAny(s, "{#") && templateCode.MatchString(s)
	}}
}

type javaDeserializationDetector struct{}

func JavaDeserializationDetector() Detector          { return javaDeserializationDetector{} }
func (javaDeserializationDetector) Name() string     { return "java_deser" }
func (javaDeserializationDetector) Category() string { return "java_deser" }
func (javaDeserializationDetector) Inspect(v *NormalizedView) []Match {
	magic := []byte{0xac, 0xed, 0, 5}
	if strings.EqualFold(strings.TrimSpace(strings.SplitN(v.Request.Header.Get("Content-Type"), ";", 2)[0]), "application/x-java-serialized-object") || bytes.HasPrefix(v.Body, magic) {
		return []Match{{"java_deser", "body", "java_serialization"}}
	}
	for _, f := range v.Fields {
		if v.expired() {
			return nil
		}
		if !v.excluded(f.Name) && strings.HasPrefix(f.RawValue, string(magic)) {
			return []Match{{"java_deser", f.Source, "java_serialization"}}
		}
	}
	return (valueDetector{"java_deser", func(s string) bool {
		return strings.HasPrefix(strings.TrimSpace(s), "rO0AB") || javaGadget.MatchString(s)
	}}).Inspect(v)
}
