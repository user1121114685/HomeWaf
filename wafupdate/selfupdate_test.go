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
	"testing"
	"time"
)

func TestUpdaterFetchMustReturnNonNilReaderCloser(t *testing.T) {
	mr := &mockRequester{}
	mr.handleRequest(
		func(url string) (io.ReadCloser, error) {
			return nil, nil
		})
	updater := createUpdater(mr)
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
	updater := createUpdater(mr)
	updater.CheckTime = 24
	updater.RandomizeTime = 24

	err := updater.BackgroundRun()
	if err != nil {
		t.Errorf("Error occurred: %#v", err)
	}
}

func TestUpdaterCheckTime(t *testing.T) {
	mr := &mockRequester{}
	mr.handleRequest(
		func(url string) (io.ReadCloser, error) {
			equals(t, "http://updates.yourdomain.com/myapp/"+plat+".json?"+ClientQuery(), url)
			return newTestReaderCloser(`{"Version":"v1.0.0","Sha256":"` + base64.StdEncoding.EncodeToString(make([]byte, 32)) + `"}`), nil
		})

	// Run test with various time
	runTestTimeChecks(t, mr, 0, 0, false)
	runTestTimeChecks(t, mr, 0, 5, true)
	runTestTimeChecks(t, mr, 1, 0, true)
	runTestTimeChecks(t, mr, 100, 100, true)
}

// Helper function to run check time tests
func runTestTimeChecks(t *testing.T, mr *mockRequester, checkTime int, randomizeTime int, expectUpdate bool) {
	updater := createUpdater(mr)
	updater.ClearUpdateState()
	updater.CheckTime = checkTime
	updater.RandomizeTime = randomizeTime

	updater.BackgroundRun()

	if updater.WantUpdate() == expectUpdate {
		t.Errorf("WantUpdate returned %v; want %v", updater.WantUpdate(), expectUpdate)
	}

	maxHrs := time.Duration(updater.CheckTime+updater.RandomizeTime) * time.Hour
	maxTime := time.Now().Add(maxHrs)

	if !updater.NextUpdate().Before(maxTime) {
		t.Errorf("NextUpdate should less than %s hrs (CheckTime + RandomizeTime) from now; now %s; next update %s", maxHrs, time.Now(), updater.NextUpdate())
	}

	if maxHrs > 0 && !updater.NextUpdate().After(time.Now()) {
		t.Errorf("NextUpdate should be after now")
	}
}

func TestUpdaterWithEmptyPayloadNoErrorNoUpdateEscapedPath(t *testing.T) {
	mr := &mockRequester{}
	mr.handleRequest(
		func(url string) (io.ReadCloser, error) {
			equals(t, "http://updates.yourdomain.com/myapp%2Bfoo/"+plat+".json?"+ClientQuery(), url)
			return newTestReaderCloser(`{"Version":"v1.0.0","Sha256":"` + base64.StdEncoding.EncodeToString(make([]byte, 32)) + `"}`), nil
		})
	updater := createUpdaterWithEscapedCharacters(mr)

	err := updater.BackgroundRun()
	if err != nil {
		t.Errorf("Error occurred: %#v", err)
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
	updater := createUpdater(mr)

	available, ver, desc, err := updater.UpdateAvailable()
	if err != nil {
		t.Errorf("Error occurred: %#v", err)
	}
	equals(t, true, available)
	equals(t, "v1.0.30", ver)
	equals(t, "fixsamebug", desc)
}

func createUpdater(mr *mockRequester) *Updater {
	return &Updater{
		CurrentVersion: "1.2",
		ApiURL:         "http://updates.yourdomain.com/",
		BinURL:         "http://updates.yourdownmain.com/",
		DiffURL:        "http://updates.yourdomain.com/",
		Dir:            "update/",
		CmdName:        "myapp", // app name
		Requester:      mr,
	}
}

func createUpdaterWithEscapedCharacters(mr *mockRequester) *Updater {
	return &Updater{
		CurrentVersion: "1.2+foobar",
		ApiURL:         "http://updates.yourdomain.com/",
		BinURL:         "http://updates.yourdownmain.com/",
		DiffURL:        "http://updates.yourdomain.com/",
		Dir:            "update/",
		CmdName:        "myapp+foo", // app name
		Requester:      mr,
	}
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
	payload, _ := json.Marshal(map[string]any{"tag_name": "v2.0.1", "body": "test release", "assets": []map[string]any{{"name": "HomeWaf_" + platform + "_2.0.1.zip", "browser_download_url": "https://publisher.example/HomeWaf.zip", "digest": "sha256:0000000000000000000000000000000000000000000000000000000000000000"}}})
	mr := &mockRequester{}
	mr.handleRequest(func(url string) (io.ReadCloser, error) { return newTestReaderCloser(string(payload)), nil })
	updater := createUpdater(mr)
	if err := updater.fetchInfoGithub(); err != nil {
		t.Fatal(err)
	}
	if updater.BinGithubURL == "" || updater.Info.Version != "v2.0.1" || len(updater.ArchiveSHA256) != 64 {
		t.Fatal("invalid release metadata")
	}
}
