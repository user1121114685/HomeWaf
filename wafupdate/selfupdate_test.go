// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package wafupdate

import (
	"HomeWaf/global"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUpdaterFetchMustReturnNonNilReaderCloser(t *testing.T) {
	mr := &mockRequester{}
	mr.handleRequest(
		func(url string) (io.ReadCloser, error) {
			return nil, nil
		})
	updater := createUpdater(t, mr)
	updater.CheckTime = 24
	updater.RandomizeTime = 24

	err := updater.BackgroundRun()

	if err != nil {
		equals(t, "Fetch was expected to return non-nil ReadCloser", err.Error())
	} else {
		t.Log("Expected an error")
		t.Fail()
	}
}

func TestUpdaterWithEmptyPayloadNoErrorNoUpdate(t *testing.T) {
	mr := &mockRequester{}
	mr.handleRequest(
		func(url string) (io.ReadCloser, error) {
			equals(t, "http://updates.yourdomain.com/myapp/"+plat+".json?"+ClientQuery(), url)
			return newTestReaderCloser(`{"Version":"v1.0.0","Sha256":"` + base64.StdEncoding.EncodeToString(make([]byte, 32)) + `"}`), nil
		})
	updater := createUpdater(t, mr)
	updater.CheckTime = 24
	updater.RandomizeTime = 24

	err := updater.BackgroundRun()
	if err != nil {
		t.Errorf("Error occurred: %#v", err)
	}
}

func TestUpdaterCheckTime(t *testing.T) {
	for _, times := range [][2]int{{0, 0}, {0, 5}, {1, 0}, {100, 100}} {
		t.Run(fmt.Sprintf("check=%d/jitter=%d", times[0], times[1]), func(t *testing.T) {
			mr := &mockRequester{}
			mr.handleRequest(func(url string) (io.ReadCloser, error) {
				equals(t, "http://updates.yourdomain.com/myapp/"+plat+".json?"+ClientQuery(), url)
				return newTestReaderCloser(`{"Version":"v1.0.0","Sha256":"` + base64.StdEncoding.EncodeToString(make([]byte, 32)) + `"}`), nil
			})
			runTestTimeChecks(t, mr, times[0], times[1])
		})
	}
}

// Helper function to run check time tests
func runTestTimeChecks(t *testing.T, mr *mockRequester, checkTime int, randomizeTime int) {
	t.Helper()
	updater := createUpdater(t, mr)
	updater.ClearUpdateState()
	t.Cleanup(updater.ClearUpdateState)
	updater.CheckTime = checkTime
	updater.RandomizeTime = randomizeTime
	started := time.Now().Truncate(time.Second)
	if err := updater.BackgroundRun(); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	next := updater.NextUpdate()
	minimum := started.Add(time.Duration(checkTime) * time.Hour)
	maximum := now.Add(time.Duration(checkTime+randomizeTime) * time.Hour)
	if next.Before(minimum) || next.After(maximum) {
		t.Errorf("NextUpdate %s outside [%s, %s]", next, minimum, maximum)
	}
	// Jitter is inclusive of zero. With CheckTime=0, an immediate check is
	// valid; deriving the expectation from the saved time covers both bounds.
	if want := !next.After(now); updater.WantUpdate() != want {
		t.Errorf("WantUpdate returned %v; want %v for next=%s", updater.WantUpdate(), want, next)
	}
}

func TestUpdaterWithEmptyPayloadNoErrorNoUpdateEscapedPath(t *testing.T) {
	mr := &mockRequester{}
	mr.handleRequest(
		func(url string) (io.ReadCloser, error) {
			equals(t, "http://updates.yourdomain.com/myapp%2Bfoo/"+plat+".json?"+ClientQuery(), url)
			return newTestReaderCloser(`{"Version":"v1.0.0","Sha256":"` + base64.StdEncoding.EncodeToString(make([]byte, 32)) + `"}`), nil
		})
	updater := createUpdaterWithEscapedCharacters(t, mr)

	err := updater.BackgroundRun()
	if err != nil {
		t.Errorf("Error occurred: %#v", err)
	}
	if mr.currentIndex != 1 {
		t.Fatalf("expected one metadata request, got %d", mr.currentIndex)
	}
}

func TestUpdateAvailable(t *testing.T) {
	mr := &mockRequester{}
	mr.handleRequest(
		func(url string) (io.ReadCloser, error) {
			equals(t, "http://updates.yourdomain.com/myapp/"+plat+".json?"+ClientQuery(), url)
			return newTestReaderCloser(`{
    "Version": "v1.0.30",
    "Sha256": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
	"Desc":"fixsamebug"
}`), nil
		})
	updater := createUpdater(t, mr)

	available, ver, desc, err := updater.UpdateAvailable()
	if err != nil {
		t.Errorf("Error occurred: %#v", err)
	}
	equals(t, true, available)
	equals(t, "v1.0.30", ver)
	equals(t, "fixsamebug", desc)
}

func createUpdater(t *testing.T, mr *mockRequester) *Updater {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir, err := filepath.Rel(filepath.Dir(exe), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return &Updater{
		CurrentVersion: "v1.0.0",
		ApiURL:         "http://updates.yourdomain.com/",
		BinURL:         "http://updates.yourdownmain.com/",
		DiffURL:        "http://updates.yourdomain.com/",
		Dir:            dir + string(filepath.Separator),
		CmdName:        "myapp", // app name
		Requester:      mr,
	}
}

func createUpdaterWithEscapedCharacters(t *testing.T, mr *mockRequester) *Updater {
	u := createUpdater(t, mr)
	u.CurrentVersion = "v1.2.0+foobar"
	u.CmdName = "myapp+foo"
	return u
}

func equals(t *testing.T, expected, actual interface{}) {
	if expected != actual {
		t.Logf("Expected: %#v got %#v\n", expected, actual)
		t.Fail()
	}
}

type testReadCloser struct {
	buffer *bytes.Buffer
}

func newTestReaderCloser(payload string) io.ReadCloser {
	return &testReadCloser{buffer: bytes.NewBufferString(payload)}
}

func (trc *testReadCloser) Read(p []byte) (n int, err error) {
	return trc.buffer.Read(p)
}

func (trc *testReadCloser) Close() error {
	return nil
}
func TestUpdater_GetHttps(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "{}") }))
	defer server.Close()
	response, err := server.Client().Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatal(response.StatusCode)
	}
}

func TestFetchInfoGithubIntegration(t *testing.T) {
	old := global.GUPDATE_GITHUB_VERSION_URL
	t.Cleanup(func() { global.GUPDATE_GITHUB_VERSION_URL = old })
	global.GUPDATE_GITHUB_VERSION_URL = "https://publisher.example/releases/latest"
	platform := map[string]string{"windows-amd64": "Windows_x86_64", "linux-amd64": "Linux_x86_64", "linux-arm64": "Linux_arm64", "darwin-amd64": "Darwin_x86_64", "darwin-arm64": "Darwin_arm64"}[plat]
	if platform == "" {
		t.Skip("release platform not supported")
	}
	extension := ".tar.gz"
	if platform == "Windows_x86_64" {
		extension = ".zip"
	}
	payload, _ := json.Marshal(map[string]any{"tag_name": "v2.0.1", "body": "test release", "assets": []map[string]any{{"name": "HomeWaf_" + platform + "_2.0.1" + extension, "browser_download_url": "https://publisher.example/HomeWaf" + extension, "digest": "sha256:0000000000000000000000000000000000000000000000000000000000000000"}}})
	mr := &mockRequester{}
	mr.handleRequest(func(url string) (io.ReadCloser, error) { return newTestReaderCloser(string(payload)), nil })
	updater := createUpdater(t, mr)
	if err := updater.fetchInfoGithub(); err != nil {
		t.Fatal(err)
	}
	if updater.BinGithubURL == "" || updater.Info.Version != "v2.0.1" || len(updater.ArchiveSHA256) != 64 {
		t.Fatal("invalid release metadata")
	}
}
