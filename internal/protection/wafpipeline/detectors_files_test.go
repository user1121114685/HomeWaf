package wafpipeline

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"net/netip"
	"net/textproto"
	"net/url"
	"strings"
	"testing"
)

func TestFileAndCommandCorpus(t *testing.T) {
	cases := map[string]struct{ bad, good []string }{
		"lfi": {[]string{"../../etc/passwd", "/etc/shadow", "/proc/self/environ", "C:\\Windows\\win.ini", "php://filter/convert.base64-encode/resource=index.php", "file:///etc/passwd", "expect://id", "/..;/..;/etc/passwd", "%252e%252e%252fetc%252fpasswd", "/WEB-INF/web.xml", "..\\..\\Windows\\system.ini", "/../../secret.txt"}, []string{"照片 (1)#%&.txt", "/assets/theme/../css/app.css", "/etc/hostsettings", "https://example.test/image.jpg", "./photos/旅行.jpg", "archive..zip", "/notes/configuration", "no path here", "/files/2026/report.pdf", "foo/bar.txt", "/devices/camera;channel=1", "percent%20literal"}},
		"rce": {[]string{"; whoami", "&& id", "| uname -a", "$(id)", "`whoami`", "; cat /etc/passwd", "; powershell -enc AAAA", "; who^ami", ";cat${IFS}/etc/passwd", "x\nping -c 1 127.0.0.1", "; cmd.exe /c dir", "| curl http://example.test"}, []string{"family & friends", "photo; edited", "a || b", "price $5", "Use `id` as the key", "cat /etc/passwd", "chmod +x script.sh", "download files", "input=hello&value=42", "2026-11-10 12:00", "a && b", "{{readable documentation}}"}},
	}
	for category, corpus := range cases {
		for _, bad := range corpus.bad {
			t.Run(category+"/bad/"+bad, func(t *testing.T) {
				r := httptest.NewRequest("GET", "http://example.test/?q="+url.QueryEscape(bad), nil)
				if got := evalDetector(t, category, r); got.Verdict.Action != Deny {
					t.Fatalf("missed %q %+v", bad, got)
				}
			})
		}
		for _, good := range corpus.good {
			t.Run(category+"/good/"+good, func(t *testing.T) {
				r := httptest.NewRequest("GET", "http://example.test/?q="+url.QueryEscape(good), nil)
				if got := evalDetector(t, category, r); got.Verdict.Action != Allow {
					t.Fatalf("false positive %q %+v", good, got)
				}
			})
		}
	}
}
func uploadRequest(t *testing.T, filename, body string) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.SetBoundary("HomeWafFixtureBoundary")
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte(body))
	w.Close()
	return &buf
}
func TestUploadCorpusNASObservationAndParserVariants(t *testing.T) {
	for _, filename := range []string{"shell.php", "shell.PHTML", "shell.php.jpg", "shell.jsp", "shell.jspx", "shell.asp", "shell.aspx", ".htaccess", "run.sh", "tool.exe", "shell.php. ", "shell.php::$DATA", "C:\\upload\\shell.php"} {
		t.Run("dangerous/"+filename, func(t *testing.T) {
			buf := uploadRequest(t, filename, "fixture")
			r := httptest.NewRequest("POST", "http://example.test/upload", buf)
			r.Header.Set("Content-Type", "multipart/form-data; boundary=HomeWafFixtureBoundary")
			if got := evalDetector(t, "upload", r); got.Verdict.Action != Deny {
				t.Fatalf("filename missed %q %+v", filename, got)
			}
		})
	}
	for _, filename := range []string{"家庭 照片.txt", "album(1)#%&.txt", "report.pdf", "README", "notes.md", "config.json", "archive.zip", "source.go", "styles.css", "bundle.js", "table.csv"} {
		t.Run("normal/"+filename, func(t *testing.T) {
			buf := uploadRequest(t, filename, "ordinary content")
			r := httptest.NewRequest("POST", "http://example.test/upload", buf)
			r.Header.Set("Content-Type", "multipart/form-data; boundary=HomeWafFixtureBoundary")
			if got := evalDetector(t, "upload", r); got.Verdict.Action != Allow {
				t.Fatalf("normal upload blocked %q %+v", filename, got)
			}
		})
	}
	for _, filename := range []string{"backup.php", "install.sh", "installer.exe"} {
		runtime, s := detectorFixture("upload", ModeObserve)
		buf := uploadRequest(t, filename, strings.Repeat("normal NAS file", 1000))
		r := httptest.NewRequest("POST", "http://example.test/webdav", buf)
		r.Header.Set("Content-Type", "multipart/form-data; boundary=HomeWafFixtureBoundary")
		got, err := runtime.Explain(s, r, netip.MustParseAddr("192.0.2.1"))
		if err != nil || got.Verdict.Action != Allow || len(got.Hits) != 1 || !got.Hits[0].Observe {
			t.Fatalf("NAS observe blocked: %+v %v", got, err)
		}
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	hdr := textproto.MIMEHeader{}
	hdr.Set("Content-Disposition", `form-data; name="file"; filename="safe.txt"; filename*=UTF-8''shell.php`)
	part, _ := w.CreatePart(hdr)
	part.Write([]byte("fixture"))
	w.Close()
	r := httptest.NewRequest("POST", "http://example.test/upload", &buf)
	r.Header.Set("Content-Type", w.FormDataContentType())
	if got := evalDetector(t, "upload", r); got.Verdict.Action != Deny {
		t.Fatal("alternate filename escaped")
	}
}

func BenchmarkFileDetectors(b *testing.B) {
	for _, d := range []Detector{LFIDetector(), RCEDetector(), UploadDetector()} {
		b.Run(d.Name(), func(b *testing.B) {
			r := httptest.NewRequest("GET", "http://example.test/?file=family%20photo.txt", nil)
			v := NewNormalizedView(r, DefaultViewLimits())
			v.Files = []FilePart{{Filename: "notes.txt", Filenames: []string{"notes.txt"}, Prefix: []byte("ordinary notes")}}
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				d.Inspect(v)
			}
		})
	}
}
