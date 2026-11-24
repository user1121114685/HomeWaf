package wafupdate

import (
	"encoding/json"
	"errors"
	"math/rand/v2"
	"os"
	"path/filepath"
	"time"

	"HomeWaf/internal/instance/global"
	"golang.org/x/mod/semver"
)

const (
	releaseScheduleVersion = 2
	releaseCheckPeriod     = 24 * time.Hour
	releaseCheckRetryMin   = 30 * time.Minute
	releaseCheckRetryMax   = 6 * time.Hour
)

type releaseSchedule struct {
	Version     int       `json:"version"`
	WindowStart time.Time `json:"window_start"`
	NextCheck   time.Time `json:"next_check"`
	RetryCheck  time.Time `json:"retry_check,omitempty"`
	Notified    string    `json:"notified_version,omitempty"`
}

func (s *releaseSchedule) scheduleWindow(start time.Time) {
	s.Version = releaseScheduleVersion
	s.WindowStart = start
	minute := 1 + rand.IntN(int(releaseCheckPeriod/time.Minute)-1)
	s.NextCheck = start.Add(time.Duration(minute) * time.Minute)
}

func releaseCheckRetryDelay() time.Duration {
	minutes := int((releaseCheckRetryMax - releaseCheckRetryMin) / time.Minute)
	return releaseCheckRetryMin + time.Duration(rand.IntN(minutes))*time.Minute
}

// CheckDue keeps checks spread across randomized windows. Failed requests get
// a separately persisted retry deadline so they do not wait for the next window.
func (m *ReleaseMonitor) CheckDue(now time.Time, notify func(current, latest string)) error {
	if global.GUPDATE_RELEASE_URL == "" &&
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
		m.nextDue = plan.NextCheck
		return nil
	}
	if plan.Version < releaseScheduleVersion {
		plan.Version = releaseScheduleVersion
		if err := writeReleaseState(file, plan); err != nil {
			return err
		}
	}
	regularDue := !now.Before(plan.NextCheck)
	retryDue := !plan.RetryCheck.IsZero() && !now.Before(plan.RetryCheck)
	if !regularDue && !retryDue {
		m.nextDue = plan.NextCheck
		if !plan.RetryCheck.IsZero() && plan.RetryCheck.Before(m.nextDue) {
			m.nextDue = plan.RetryCheck
		}
		return nil
	}
	if regularDue {
		// After downtime check once, then skip any missed windows.
		windows := now.Sub(plan.WindowStart)/releaseCheckPeriod + 1
		plan.scheduleWindow(plan.WindowStart.Add(windows * releaseCheckPeriod))
	}
	plan.RetryCheck = now.Add(releaseCheckRetryDelay())
	if err := writeReleaseState(file, plan); err != nil {
		return err
	}
	m.nextDue = plan.RetryCheck
	if plan.NextCheck.Before(m.nextDue) {
		m.nextDue = plan.NextCheck
	}
	check, err := m.Check(false)
	if err != nil {
		return err
	}
	plan.RetryCheck = time.Time{}
	m.nextDue = plan.NextCheck
	if check.Version == "" || check.Prerelease {
		return writeReleaseState(file, plan)
	}
	version := semver.Canonical(normalizedVersion(check.Version))
	if semver.Compare(version, plan.Notified) <= 0 {
		return writeReleaseState(file, plan)
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
