package iplocation

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type openSourceTransport func(*http.Request) (*http.Response, error)

func (fn openSourceTransport) RoundTrip(r *http.Request) (*http.Response, error) { return fn(r) }

func setupOpenSourceTest(t *testing.T, data []byte, badHash bool) *int {
	t.Helper()
	prior := currentUpgradeCfg()
	t.Cleanup(func() { ConfigureUpgrader(prior) })
	commit := strings.Repeat("a", 40)
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(data))
	h.Write(data)
	blob := hex.EncodeToString(h.Sum(nil))
	if badHash {
		blob = strings.Repeat("b", 40)
	}
	count := new(int)
	ConfigureUpgrader(UpgradeConfig{
		ClientQuery: func() string { return "v=private&u=must-not-leak" },
		NewClient: func() *http.Client {
			return &http.Client{Transport: openSourceTransport(func(r *http.Request) (*http.Response, error) {
				*count++
				if strings.Contains(r.URL.RawQuery, "must-not-leak") {
					t.Fatal("client ID leaked to public source")
				}
				var body []byte
				switch {
				case r.URL.String() == openSourceAPI+"/git/ref/heads/master":
					body = []byte(`{"object":{"sha":"` + commit + `"}}`)
				case r.URL.String() == openSourceAPI+"/contents/data?ref="+commit:
					body, _ = json.Marshal([]map[string]any{{"name": "ip2region_v4.xdb", "sha": blob, "size": len(data)}})
				case r.URL.String() == openSourceRaw+commit+"/data/ip2region_v4.xdb":
					body = data
				default:
					return nil, fmt.Errorf("unexpected outbound URL %s", r.URL)
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(body)), ContentLength: int64(len(body)), Request: r}, nil
			})}
		},
	})
	return count
}

func TestOpenSourceCheckAndVerifiedDownload(t *testing.T) {
	data, err := os.ReadFile("../cmd/homewaf/exedata/ip2region.xdb")
	if err != nil {
		t.Fatal(err)
	}
	count := setupOpenSourceTest(t, data, false)
	dir := t.TempDir()
	info, err := CheckUpgrade(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(info.SourceLabel, "开源") {
		t.Fatal(info.SourceLabel)
	}
	_, err = CheckUpgrade(dir)
	if err != nil {
		t.Fatal(err)
	}
	if *count != 2 {
		t.Fatalf("metadata should be cached, requests=%d", *count)
	}
	if err = ApplyUpgrade(dir, "ip2region_v4", func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	info, err = CheckUpgrade(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range info.Files {
		if f.Key == "ip2region_v4" {
			if f.NeedUpdate || !f.LocalExists || f.LocalVersion != f.LatestVersion {
				t.Fatalf("bad version state: %+v", f)
			}
		}
	}
	if *count != 3 {
		t.Fatalf("unexpected requests: %d", *count)
	}
}

func TestFailedDownloadKeepsExistingDatabase(t *testing.T) {
	data, err := os.ReadFile("../cmd/homewaf/exedata/ip2region.xdb")
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"checksum", "invalid-xdb", "reload"} {
		t.Run(mode, func(t *testing.T) {
			payload := data
			if mode == "invalid-xdb" {
				payload = bytes.Repeat([]byte("bad database"), 30)
			}
			setupOpenSourceTest(t, payload, mode == "checksum")
			dir := t.TempDir()
			dst := filepath.Join(dir, "ip2region.xdb")
			old := []byte("previous database remains recoverable")
			if err := os.WriteFile(dst, old, 0600); err != nil {
				t.Fatal(err)
			}
			writeLocalVersion(dir, "ip2region_v4", "old-version")
			reload := func() error {
				if mode == "reload" {
					return errors.New("test reload error")
				}
				return nil
			}
			if err := ApplyUpgrade(dir, "ip2region_v4", reload); err == nil {
				t.Fatal("expected failure")
			}
			got, err := os.ReadFile(dst)
			if err != nil || !bytes.Equal(got, old) {
				t.Fatalf("old file lost: %v", err)
			}
			if readLocalVersions(dir)["ip2region_v4"] != "old-version" {
				t.Fatal("version advanced after failed update")
			}
		})
	}
}

func TestOpenSourceRateLimitReturnsLocalState(t *testing.T) {
	prior := currentUpgradeCfg()
	t.Cleanup(func() { ConfigureUpgrader(prior) })
	ConfigureUpgrader(UpgradeConfig{NewClient: func() *http.Client {
		return &http.Client{Transport: openSourceTransport(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 403, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{}`)), Request: r}, nil
		})}
	}})
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ip2region.xdb"), []byte("local data"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := CheckUpgrade(dir)
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("expected specific HTTP error, got %v", err)
	}
	if info == nil || len(info.Files) != len(KnownDatabases) {
		t.Fatal("local file state missing on network failure")
	}
	for _, f := range info.Files {
		if f.Key == "ip2region_v4" && !f.LocalExists {
			t.Fatal("existing file hidden by network failure")
		}
	}
}
