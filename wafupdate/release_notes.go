package wafupdate

import (
	"HomeWaf/global"
	"HomeWaf/model"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

type ReleaseNote = model.VersionReleaseNote

var noteBullet = regexp.MustCompile(`^(?:[-*+] |\d+\. )`)
var noteCommit = regexp.MustCompile(`(?i)^(?:\[[a-f0-9]{7,40}\]\([^)]*\)|[a-f0-9]{7,40})\s+`)
var notePrefix = regexp.MustCompile(`(?i)^(?:feat|fix|perf|refactor|chore|docs|test|ci|build)(?:\([^)]*\))?!?:\s*`)
var noteLink = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`)
var noteHeading = regexp.MustCompile(`(?i)^(?:changelog|what'?s changed|更新内容|更新说明|(?:HomeWaf\s+)?v\d+\.\d+\.\d+[^\s]*)$`)
var releaseMetadataRequester = HTTPRequester{Client: &http.Client{Transport: defaultHTTPClient.Transport, Timeout: 5 * time.Second}}

// Release descriptions are display text, never executable HTML.
func releaseNoteItems(body string) []string {
	items := []string{}
	seen := map[string]bool{}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(noteBullet.ReplaceAllString(strings.TrimSpace(line), ""), "#"))
		if line == "" || noteHeading.MatchString(line) {
			continue
		}
		line = noteCommit.ReplaceAllString(line, "")
		line = notePrefix.ReplaceAllString(line, "")
		line = strings.TrimSpace(noteLink.ReplaceAllString(line, "$1"))
		if line != "" && !seen[line] {
			items = append(items, line)
			seen[line] = true
		}
	}
	return items
}

// RecentReleaseNotes describes only versions newer than this binary and no newer
// than the selected update. Failure preserves the already verified target notes.
func (u *Updater) RecentReleaseNotes(channel string) ([]ReleaseNote, error) {
	fallback := []ReleaseNote{{Version: u.Info.Version, Items: releaseNoteItems(u.Info.Desc)}}
	endpoint := global.GUPDATE_RELEASE_URL
	allowPrerelease := channel == "github"
	if allowPrerelease {
		endpoint = global.GUPDATE_GITHUB_VERSION_URL
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return fallback, err
	}
	index := strings.LastIndex(parsed.Path, "/releases/")
	if index < 0 {
		return fallback, nil
	}
	parsed.Path = parsed.Path[:index] + "/releases"
	parsed.RawPath, parsed.Fragment = "", ""
	query := parsed.Query()
	query.Set("per_page", "20")
	query.Set("page", "1")
	parsed.RawQuery = query.Encode()
	history := *u
	if history.Requester == nil {
		history.Requester = &releaseMetadataRequester
	}
	reader, err := history.fetch(parsed.String())
	if err != nil {
		return fallback, err
	}
	defer reader.Close()
	var releases []struct {
		Version    string `json:"tag_name"`
		Name       string `json:"name"`
		Body       string `json:"body"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	if err := json.NewDecoder(io.LimitReader(reader, 2<<20)).Decode(&releases); err != nil {
		return fallback, fmt.Errorf("invalid release history: %w", err)
	}
	notes := append([]ReleaseNote{}, fallback...)
	seen := map[string]bool{normalizedVersion(u.Info.Version): true}
	for _, release := range releases {
		version := normalizedVersion(release.Version)
		if seen[version] || release.Draft || !semver.IsValid(version) ||
			strings.Contains(strings.ToLower(release.Name+" "+release.Version), "debug") ||
			(!allowPrerelease && (release.Prerelease || semver.Prerelease(version) != "")) ||
			semver.Compare(version, normalizedVersion(u.CurrentVersion)) <= 0 ||
			semver.Compare(version, normalizedVersion(u.Info.Version)) > 0 {
			continue
		}
		seen[version] = true
		notes = append(notes, ReleaseNote{Version: version, Items: releaseNoteItems(release.Body)})
	}
	sort.Slice(notes, func(i, j int) bool { return semver.Compare(notes[i].Version, notes[j].Version) > 0 })
	return notes, nil
}
