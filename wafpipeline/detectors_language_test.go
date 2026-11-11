package wafpipeline

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

var languageCorpus = map[string][]string{
	"java_inject": {`${jndi:ldap://fixture.invalid/a}`, `${JNDI:rmi://fixture.invalid/a}`, `${${lower:j}ndi:ldap://fixture.invalid/a}`, `${${::-j}${::-n}${::-d}${::-i}:ldap://fixture.invalid/a}`, `Runtime.getRuntime().exec("id")`, `new java.lang.ProcessBuilder("id")`, `T(java.lang.Runtime).getRuntime()`, `@java.lang.Runtime@getRuntime()`, `#_memberAccess['allowStaticMethodAccess']=true`, `#context['xwork.MethodAccessor.denyMethodExecution']=false`, `com.sun.rowset.JdbcRowSetImpl`, `%2524%257Bjndi%253Aldap%253A%252F%252Ffixture.invalid%252Fa%257D`},
	"java_deser":  {"\xac\xed\x00\x05fixture", "rO0ABQ==", "rO0ABXfixture", " rO0ABQfixture", "org.apache.commons.collections.functors.InvokerTransformer", "org.apache.commons.collections4.functors.InvokerTransformer", "org.apache.commons.collections.functors.ChainedTransformer", "com.sun.org.apache.xalan.internal.xsltc.trax.TemplatesImpl", "org.codehaus.groovy.runtime.ConvertedClosure", "org.springframework.beans.factory.ObjectFactory"},
	"php_deser":   {`O:8:"stdClass":0:{}`, `O:4:"User":1:{s:1:"a";s:1:"b";}`, `C:4:"Test":3:{abc}`, `a:1:{i:0;s:1:"x";}`, `phar://archive.phar/file`, `PHAR://archive.phar/file`, `O:3:"Foo":0:{}`, `a:0:{}`, `O:6:"Gadget":1:{s:1:"x";N;}`, `%254F%253A8%253A%2522stdClass%2522%253A0%253A%257B%257D`},
	"php_inject":  {`<?php echo 1;`, `<?=system('id');?>`, `php://input`, `data://text/plain,<?php echo 1;`, `eval($_POST['q'])`, `assert($_GET['q'])`, `system('id')`, `shell_exec("id")`, `eval(base64_decode('aWQ='))`, `passthru($cmd)`, `<?PHP echo 1;`, `%253C%253Fphp%2520echo%25201%253B`},
	"asp_inject":  {`<%=1+1%>`, `<%Execute("code")%>`, `<%eval("1+1")%>`, `<%Response.Write(1)%>`, `<%Server.CreateObject("WScript.Shell")%>`, `<%Dim x%>`, `Server.CreateObject("WScript.Shell")`, `Server.CreateObject("ADODB.Stream")`, `Execute("code")`, `Eval('1+1')`, `<% response.write(1)%>`, `%253C%2525%253D1%252B1%2525%253E`},
	"ssti":        {`{{7*7}}`, `${7*7}`, `#{7*7}`, `{{config}}`, `{{''.__class__}}`, `{{request.__globals__}}`, `{{''.__class__.__mro__}}`, `{{''.__subclasses__()}}`, `<#assign x="freemarker.template.utility.Execute"?new()>`, `#set($x=$class.forName("java.lang.Runtime"))`, `${T(java.lang.Runtime).getRuntime()}`, `%257B%257B7%252A7%257D%257D`},
}
var languageBenign = []string{"family photo album", "hello Java", "PHP documentation", "ASP.NET Core introduction", "{{ordinary label}}", "${ordinary label}", "<% is a template marker", "a:1 is a chart coordinate", "JSON key @type documents an object", "Runtime statistics", "ProcessBuilder usage overview", "base64: aGVsbG8=", "<p>中文笔记</p>", "O'Reilly books"}

func TestLanguageCorpusAndEmbeddedFields(t *testing.T) {
	for category, bad := range languageCorpus {
		t.Run(category, func(t *testing.T) {
			for _, value := range bad {
				t.Run(value, func(t *testing.T) {
					r := httptest.NewRequest("GET", "http://example.test/?q="+url.QueryEscape(value), nil)
					if got := evalDetector(t, category, r); got.Verdict.Action != Deny {
						t.Fatalf("missed %q %+v", value, got)
					}
				})
			}
			for _, value := range languageBenign {
				r := httptest.NewRequest("GET", "http://example.test/?q="+url.QueryEscape(value), nil)
				if got := evalDetector(t, category, r); got.Verdict.Action != Allow {
					t.Fatalf("false positive %q %+v", value, got)
				}
			}
			value := bad[len(bad)-1]
			data, _ := json.Marshal(map[string]any{"outer": map[string]string{"nested": value}})
			r := httptest.NewRequest("POST", "http://example.test/", strings.NewReader(string(data)))
			r.Header.Set("Content-Type", "application/json")
			if got := evalDetector(t, category, r); got.Verdict.Action != Deny {
				t.Fatalf("nested JSON missed: %+v", got)
			}
		})
	}
}
func TestJavaBinaryBodyAndMIME(t *testing.T) {
	for _, ct := range []string{"application/octet-stream", "application/x-java-serialized-object"} {
		r := httptest.NewRequest("POST", "http://example.test/", strings.NewReader("\xac\xed\x00\x05fixture"))
		r.Header.Set("Content-Type", ct)
		if got := evalDetector(t, "java_deser", r); got.Verdict.Action != Deny {
			t.Fatal("binary serialized body missed")
		}
	}
}

// Guard optimizations must preserve every regexp alternative, including forms
// without braces, Unicode case folding, and nested lookup replacements.
func TestLanguageFastPathsPreserveOriginalMatchers(t *testing.T) {
	d := SSTIDetector().(valueDetector)
	values := append([]string{}, languageCorpus["ssti"]...)
	values = append(values, languageBenign...)
	values = append(values, "#SET ( $x.forName('java.lang.Runtime'))", "<#aſſign x=Runtime>", "{{"+strings.Repeat("x", 512)+"config", "${${lower:j}${upper:n}${::-d}i:ldap://fixture.invalid/a}")
	for _, raw := range values {
		value := NormalizeValue(raw)
		if got, want := d.match(value), templateCode.MatchString(value); got != want {
			t.Fatalf("template guard changed match for %q: %v != %v", raw, got, want)
		}
	}
	for _, raw := range append(append([]string{}, languageCorpus["java_inject"]...), values...) {
		value := NormalizeValue(raw)
		want := value
		for i := 0; i < 4; i++ {
			next := lookupDefault.ReplaceAllString(lookupCase.ReplaceAllString(want, "$1"), "$1")
			if next == want {
				break
			}
			want = next
		}
		if got := foldLookups(value); got != want {
			t.Fatalf("lookup guard changed %q: %q != %q", raw, got, want)
		}
	}
}
func BenchmarkLanguageDetectors(b *testing.B) {
	for _, d := range []Detector{JavaInjectionDetector(), JavaDeserializationDetector(), PHPDeserializationDetector(), PHPInjectionDetector(), ASPInjectionDetector(), SSTIDetector()} {
		b.Run(d.Name(), func(b *testing.B) {
			r := httptest.NewRequest("GET", "http://example.test/?q=family+photo+album", nil)
			v := NewNormalizedView(r, DefaultViewLimits())
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				d.Inspect(v)
			}
		})
	}
}
