package wafupdate

import (
	"encoding/json"
	"errors"
	"math/rand/v2"
	"os"
	"path/filepath"
	"time"

	"HomeWaf/global"
	"golang.org/x/mod/semver"
)

const releaseCheckPeriod = 48 * time.Hour

type releaseSchedule struct {
	WindowStart time.Time `json:"window_start"`
	NextCheck   time.Time `json:"next_check"`
	Notified    string    `json:"notified_version,omitempty"`
}

func (s *releaseSchedule) scheduleWindow(start time.Time) {
	s.WindowStart = start
	// Leave a full scheduler tick at the end so the chosen minute belongs to this window.
	minute := 1 + rand.IntN(int(releaseCheckPeriod/time.Minute)-1)
	s.NextCheck = start.Add(time.Duration(minute) * time.Minute)
}

// CheckDue reserves the next window before contacting the publisher. Restarts,
// API reads and network failures cannot cause a second automatic check in this window.
func (m *ReleaseMonitor) CheckDue(now time.Time, notify func(current, latest string)) error {
	if global.GUPDATE_VERSION_URL == "" && global.GUPDATE_RELEASE_URL == "" &&
		(global.GUPDATE_GITHUB_VERSION_URL == "" || global.CurrentSettings().GCONFIG_CHECK_BETA_VERSION != 1) {
		return nil
	}
	m.planMu.Lock()
	defer m.planMu.Unlock()
	if now.Before(m.nextDue) {
		return nil
	}
	file := filepath.Join(m.dir, "release-check-schedule.json")
	var plan releaseSchedule
	err := readReleaseState(file, &plan)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if plan.NextCheck.IsZero() {
		plan.scheduleWindow(now)
		if err := writeReleaseState(file, plan); err != nil {
			return err
		}
	}
	if now.Before(plan.NextCheck) {
		m.nextDue = plan.NextCheck
		return nil
	}
	// After downtime check once, then skip any missed windows.
	windows := now.Sub(plan.WindowStart)/releaseCheckPeriod + 1
	plan.scheduleWindow(plan.WindowStart.Add(windows * releaseCheckPeriod))
	if err := writeReleaseState(file, plan); err != nil {
		return err
	}
	m.nextDue = plan.NextCheck
	check, err := m.Check(false)
	if err != nil || check.Version == "" || check.Prerelease {
		return err
	}
	version := semver.Canonical(normalizedVersion(check.Version))
	if semver.Compare(version, plan.Notified) <= 0 {
		return nil
	}
	notify(check.Current, check.Version)
	plan.Notified = version
	return writeReleaseState(file, plan)
}

func readReleaseState(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, value)
}

func writeReleaseState(path string, value any) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".release-check-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	err = json.NewEncoder(tmp).Encode(value)
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
