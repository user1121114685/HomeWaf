package wafmangeweb

import (
	"bytes"
	"io/fs"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"HomeWaf/global"
	"HomeWaf/public"
	"HomeWaf/wafmangeweb/static"
	"github.com/gin-gonic/gin"
	"golang.org/x/net/html"
)

func TestBundledConsoleFavicons(t *testing.T) {
	if _, err := fs.Stat(public.Public, "index.html"); os.IsNotExist(err) {
		t.Skip("requires the bundled console built by scripts/build.mjs --web-only")
	}
	expected := map[string][]byte{}
	for mime, file := range map[string]string{
		"image/svg+xml": "../web/src/assets/homewaf-logo.svg",
		"image/x-icon":  "../web/public/favicon.ico",
	} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		expected[mime] = data
	}
	oldHTML, oldEnabled, oldPath := global.GWAF_LOCAL_INDEX_HTML, global.GWAF_SECURITY_ENTRY_ENABLE, global.GWAF_SECURITY_ENTRY_PATH
	t.Cleanup(func() {
		global.GWAF_LOCAL_INDEX_HTML, global.GWAF_SECURITY_ENTRY_ENABLE, global.GWAF_SECURITY_ENTRY_PATH = oldHTML, oldEnabled, oldPath
	})
	router := gin.New()
	static.Static(router, router.NoRoute)
	handler := securityPathHandler{inner: router}
	for _, prefix := range []string{"/", "/test-entry/"} {
		global.GWAF_SECURITY_ENTRY_ENABLE, global.GWAF_SECURITY_ENTRY_PATH = prefix != "/", strings.Trim(prefix, "/")
		indexURL := &url.URL{Scheme: "http", Host: "console.test", Path: prefix}
		index := httptest.NewRecorder()
		handler.ServeHTTP(index, httptest.NewRequest("GET", indexURL.String(), nil))
		doc, err := html.Parse(index.Body)
		if err != nil {
			t.Fatal(err)
		}
		seen := map[string]bool{}
		for node := range doc.Descendants() {
			if node.Type != html.ElementNode || node.Data != "link" {
				continue
			}
			attrs := map[string]string{}
			for _, attr := range node.Attr {
				attrs[attr.Key] = attr.Val
			}
			if attrs["rel"] != "icon" {
				continue
			}
			iconURL, err := indexURL.Parse(attrs["href"])
			if err != nil {
				t.Fatal(err)
			}
			icon := httptest.NewRecorder()
			handler.ServeHTTP(icon, httptest.NewRequest("GET", iconURL.String(), nil))
			mime := attrs["type"]
			if icon.Code != 200 || icon.Header().Get("Content-Type") != mime || !bytes.Equal(icon.Body.Bytes(), expected[mime]) {
				t.Fatalf("%s did not serve the %s brand asset (HTTP %d, %q)", iconURL.Path, mime, icon.Code, icon.Header().Get("Content-Type"))
			}
			seen[mime] = true
		}
		for mime := range expected {
			if !seen[mime] {
				t.Fatalf("%s is missing its %s icon", indexURL, mime)
			}
		}
	}
}
