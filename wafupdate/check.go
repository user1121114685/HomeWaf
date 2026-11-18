package wafupdate

import (
	"HomeWaf/common/zlog"
	"HomeWaf/global"
	"HomeWaf/utils"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/mod/semver"
)

// ManifestURL is empty when the official channel uses GitHub Releases.
func ManifestURL() string {
	root := global.GUPDATE_VERSION_URL
	if root != "" && (global.GWAF_RUNTIME_WIN7_VERSION == "true" || utils.IsSupportedWindows7Version()) {
		return root + "win7/"
	}
	return root
}

type ReleaseCheck struct {
	Current, Version, Description, Channel string
	Notes                                  []ReleaseNote
	Message, NotesWarning, CheckWarning    string
	Prerelease, Unversioned                bool
}

type checkSource struct {
	Current, Manifest, Release, Beta string
}

type checkSnapshot struct {
	Source checkSource
	Result ReleaseCheck
	Error  string
}

type ReleaseMonitor struct {
	dir      string
	mu       sync.Mutex
	snapshot *checkSnapshot
	planMu   sync.Mutex
	nextDue  time.Time
}

var SoftwareReleases = new(ReleaseMonitor)

func NewReleaseMonitor(dataDir string) *ReleaseMonitor { return &ReleaseMonitor{dir: dataDir} }

func currentCheckSource() checkSource {
	source := checkSource{Current: global.GWAF_RELEASE_VERSION, Manifest: ManifestURL(), Release: global.GUPDATE_RELEASE_URL}
	if global.CurrentSettings().GCONFIG_CHECK_BETA_VERSION == 1 {
		source.Beta = global.GUPDATE_GITHUB_VERSION_URL
	}
	return source
}

// Cached checks include first login and reconnects. Only a manual check or the
// active worker's scheduled check can contact the publisher.
func (m *ReleaseMonitor) Check(cached bool) (ReleaseCheck, error) {
	source := currentCheckSource()
	m.mu.Lock()
	defer m.mu.Unlock()
	file := filepath.Join(m.dir, "release-check-result.json")
	if m.snapshot == nil {
		var stored checkSnapshot
		if m.dir != "" {
			if err := readReleaseState(file, &stored); err != nil && !errors.Is(err, os.ErrNotExist) {
				if cached {
					return ReleaseCheck{}, err
				}
				zlog.Warn("读取上次版本检查结果失败", err.Error())
			}
		}
		m.snapshot = &stored
	}
	if cached {
		if source != m.snapshot.Source || m.snapshot.Result.Current == "" && m.snapshot.Error == "" {
			return ReleaseCheck{Current: source.Current, Message: "尚未检查更新，可手动检查或等待自动检查"}, nil
		}
		if m.snapshot.Error != "" && m.snapshot.Result.Version == "" {
			return ReleaseCheck{}, errors.New(m.snapshot.Error)
		}
		return m.snapshot.Result, nil
	}
	result, err := source.check()
	stored := checkSnapshot{Source: source, Result: result}
	if err != nil {
		stored.Error = err.Error()
		if source == m.snapshot.Source && m.snapshot.Result.Version != "" {
			stored.Result = m.snapshot.Result
			stored.Result.CheckWarning = "最近一次检查失败，当前显示上次发现的版本。"
		}
	}
	m.snapshot = &stored
	if m.dir != "" {
		if saveErr := writeReleaseState(file, stored); saveErr != nil {
			zlog.Warn("保存版本检查结果失败", saveErr.Error())
		}
	}
	return result, err
}

func (s checkSource) check() (ReleaseCheck, error) {
	u := &Updater{CurrentVersion: s.Current, ApiURL: s.Manifest, CmdName: "homewaf_update", Requester: &releaseMetadataRequester}
	result := ReleaseCheck{Current: s.Current, Channel: "official", Message: "当前已是最新版本"}
	available, version, desc, err := u.UpdateAvailable()
	if errors.Is(err, ErrUnversionedBuild) {
		result.Unversioned, result.Message = true, err.Error()
		return result, nil
	}
	if errors.Is(err, ErrNoPublishedRelease) {
		result.Message, err = "HomeWaf 更新源尚未发布正式版本，发布后即可检查更新", nil
	}
	if !available && s.Beta != "" {
		ok, beta, body, betaErr := u.UpdateAvailableWithChannel("github")
		if ok {
			available, version, desc, err = true, beta, body, nil
			result.Channel = "github"
		} else if betaErr != nil && !errors.Is(betaErr, ErrNoPublishedRelease) {
			err = betaErr
		}
	}
	if err != nil || !available {
		return result, err
	}
	result.Version, result.Description, result.Message = version, desc, "有新版本"
	result.Prerelease = result.Channel == "github" || semver.Prerelease(normalizedVersion(version)) != ""
	result.Notes, err = u.RecentReleaseNotes(result.Channel)
	if err != nil {
		result.NotesWarning = "历史版本说明暂时无法读取，当前仅显示目标版本的更新内容。"
	}
	return result, nil
}
