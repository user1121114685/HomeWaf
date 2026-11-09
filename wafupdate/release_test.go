package wafupdate

import (
	"HomeWaf/global"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testRelease(t *testing.T) map[string]any {
	t.Helper()
	platform := map[string]string{"windows-amd64": "Windows_x86_64", "linux-amd64": "Linux_x86_64", "linux-arm64": "Linux_arm64", "darwin-amd64": "Darwin_x86_64", "darwin-arm64": "Darwin_arm64"}[plat]
	if platform == "" {
		t.Skip("unsupported release platform")
	}
	ext := ".tar.gz"
	if strings.HasPrefix(plat, "windows-") {
		ext = ".zip"
	}
	name := "HomeWaf_" + platform + "_2.0.1" + ext
	return map[string]any{"tag_name": "v2.0.1", "body": "release notes", "assets": []map[string]any{{"name": name, "browser_download_url": "https://github.com/user1121114685/HomeWaf/releases/download/v2.0.1/" + name, "digest": "sha256:" + strings.Repeat("0", 64)}}}
}

func withReleaseSource(t *testing.T) {
	t.Helper()
	old := global.GUPDATE_RELEASE_URL
	global.GUPDATE_RELEASE_URL = "https://api.github.com/repos/user1121114685/HomeWaf/releases/latest"
	t.Cleanup(func() { global.GUPDATE_RELEASE_URL = old })
}

func releaseRequester(t *testing.T, payload map[string]any) *mockRequester {
	t.Helper()
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	mr := &mockRequester{}
	mr.handleRequest(func(endpoint string) (io.ReadCloser, error) {
		if endpoint != global.GUPDATE_RELEASE_URL {
			t.Fatalf("unexpected source %s", endpoint)
		}
		return io.NopCloser(bytes.NewReader(b)), nil
	})
	return mr
}

func TestDefaultReleaseSource(t *testing.T) {
	withReleaseSource(t)
	u := &Updater{CurrentVersion: "v2.0.0-dev", Requester: releaseRequester(t, testRelease(t))}
	ok, version, desc, err := u.UpdateAvailable()
	if err != nil || !ok || version != "v2.0.1" || desc != "release notes" {
		t.Fatalf("%v %s %s %v", ok, version, desc, err)
	}
}

func TestReleaseNoDowngradeOrRedownload(t *testing.T) {
	withReleaseSource(t)
	for _, current := range []string{"v2.0.1", "2.0.1", "v2.1.0"} {
		t.Run(current, func(t *testing.T) {
			mr := releaseRequester(t, testRelease(t))
			u := &Updater{CurrentVersion: current, Requester: mr, OnSuccessfulUpdate: func() { t.Fatal("must not replace current binary") }}
			if err := u.UpdateWithChannel("official"); err != nil {
				t.Fatal(err)
			}
			if mr.currentIndex != 1 {
				t.Fatal("download requested without newer release")
			}
		})
	}
}

func TestReleaseMetadataValidation(t *testing.T) {
	withReleaseSource(t)
	for _, scenario := range []string{"draft", "prerelease", "beta-tag", "debug", "bad-version", "missing-asset", "missing-digest", "bad-digest", "http-asset", "wrong-platform"} {
		t.Run(scenario, func(t *testing.T) {
			r := testRelease(t)
			a := r["assets"].([]map[string]any)[0]
			switch scenario {
			case "draft":
				r["draft"] = true
			case "prerelease":
				r["prerelease"] = true
			case "beta-tag":
				r["tag_name"] = "v2.0.1-beta.1"
			case "debug":
				r["name"] = "DEBUG build"
			case "bad-version":
				r["tag_name"] = "nightly"
			case "missing-asset":
				r["assets"] = []any{}
			case "missing-digest":
				delete(a, "digest")
			case "bad-digest":
				a["digest"] = "md5:" + strings.Repeat("0", 64)
			case "http-asset":
				a["browser_download_url"] = "http://example.test/update.zip"
			case "wrong-platform":
				a["name"] = "OtherProduct.zip"
			}
			u := &Updater{CurrentVersion: "v2.0.0", Requester: releaseRequester(t, r)}
			if ok, _, _, err := u.UpdateAvailable(); err == nil || ok || u.BinGithubURL != "" {
				t.Fatalf("unsafe metadata accepted: %v %v", ok, err)
			}
		})
	}
}

func TestReleaseHTTPFailures(t *testing.T) {
	for _, status := range []int{404, 403, 429, 500} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
		u := &Updater{}
		err := u.fetchReleaseInfo(server.URL, false)
		server.Close()
		if err == nil {
			t.Fatalf("HTTP %d was ignored", status)
		}
		if errors.Is(err, ErrNoPublishedRelease) != (status == 404) {
			t.Fatalf("wrong HTTP %d classification: %v", status, err)
		}
	}
}

func TestReleaseHashMismatchStopsBeforeExtraction(t *testing.T) {
	withReleaseSource(t)
	mr := releaseRequester(t, testRelease(t))
	mr.handleRequest(func(string) (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("corrupt archive")), nil })
	u := &Updater{CurrentVersion: "v2.0.0", Requester: mr, OnSuccessfulUpdate: func() { t.Fatal("corrupt archive installed") }}
	if err := u.Update(); !errors.Is(err, ErrHashMismatch) {
		t.Fatalf("got %v", err)
	}
}
