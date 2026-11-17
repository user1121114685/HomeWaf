package wafupdate

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestRecentReleaseNotesKeepVersionRangeAndRemoveCommitMetadata(t *testing.T) {
	withReleaseSource(t)
	payload := []map[string]any{
		{"tag_name": "v2.0.1", "body": "already installed"},
		{"tag_name": "v2.0.3", "body": "duplicate target"},
		{"tag_name": "v2.0.2", "body": "## Changelog\n* " + strings.Repeat("b", 40) + " fix(ui): Repair certificate selection\n* feat: Add service diagnostics"},
		{"tag_name": "v2.0.4", "body": "newer than selected update"},
		{"tag_name": "v2.0.3-beta.1", "prerelease": true, "body": "beta"},
		{"tag_name": "v2.0.2+debug", "name": "DEBUG", "body": "debug"},
		{"tag_name": "v2.0.2-draft", "draft": true, "body": "draft"},
	}
	data, _ := json.Marshal(payload)
	requester := &mockRequester{}
	requester.handleRequest(func(endpoint string) (io.ReadCloser, error) {
		if !strings.HasSuffix(endpoint, "/releases?page=1&per_page=20") {
			t.Fatalf("unexpected history endpoint %s", endpoint)
		}
		return io.NopCloser(strings.NewReader(string(data))), nil
	})
	u := &Updater{CurrentVersion: "v2.0.1+local.fixture", Requester: requester}
	u.Info.Version, u.Info.Desc = "v2.0.3", "HomeWaf v2.0.3\n## Changelog\n* "+strings.Repeat("a", 40)+" fix: Restore request-header inspection"
	notes, err := u.RecentReleaseNotes("official")
	if err != nil || len(notes) != 2 || notes[0].Version != "v2.0.3" || notes[1].Version != "v2.0.2" {
		t.Fatalf("wrong release groups: %+v %v", notes, err)
	}
	if len(notes[0].Items) != 1 || notes[0].Items[0] != "Restore request-header inspection" || len(notes[1].Items) != 2 {
		t.Fatalf("raw commit metadata leaked into notes: %+v", notes)
	}
}

func TestReleaseHistoryFailurePreservesVerifiedTargetNotes(t *testing.T) {
	withReleaseSource(t)
	for _, malformed := range []bool{false, true} {
		requester := &mockRequester{}
		requester.handleRequest(func(string) (io.ReadCloser, error) {
			if malformed {
				return io.NopCloser(strings.NewReader("invalid")), nil
			}
			return nil, errors.New("history unavailable")
		})
		u := &Updater{CurrentVersion: "v2.0.1", Requester: requester}
		u.Info.Version, u.Info.Desc = "v2.0.3", "Target release notes"
		notes, err := u.RecentReleaseNotes("official")
		if err == nil || len(notes) != 1 || notes[0].Items[0] != "Target release notes" {
			t.Fatalf("history failure lost target notes: %+v %v", notes, err)
		}
	}
}
