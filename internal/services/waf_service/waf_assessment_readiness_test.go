package waf_service

import (
	"HomeWaf/internal/protection/camouflage"
	"HomeWaf/internal/protection/wafpipeline"
	"context"
	"net"
	"testing"
)

func TestAssessmentProfilesRequireSavedEnabledRunning(t *testing.T) {
	config := camouflage.DefenseConfig{Profiles: []camouflage.ProfileConfig{{ID: "ready", Enabled: true, Port: 1234}, {ID: "disabled", Port: 1235}, {ID: "failed", Enabled: true, Port: 1236}}}
	status := []camouflage.ProfileStatus{{ID: "ready", Enabled: true, Running: true, Port: 1234}, {ID: "disabled", Enabled: true, Running: true, Port: 1235}, {ID: "failed", Enabled: true, Port: 1236}, {ID: "unsaved", Enabled: true, Running: true, Port: 1237}}
	rows := assessmentReadyProfiles(config, status)
	if len(rows) != 3 || !rows[0].Running || rows[1].Running || rows[2].Running {
		t.Fatalf("readiness mismatch: %+v", rows)
	}
	config.Profiles[0].Port++
	if assessmentReadyProfiles(config, status)[0].Running {
		t.Fatal("stale runtime port counted")
	}
}
func TestAssessmentProbeFindsUnexpectedListener(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	config := wafpipeline.PortConfig{Port: uint16(listener.Addr().(*net.TCPAddr).Port), ListenEnabled: true, Policy: wafpipeline.DefaultPolicy()}
	config.Policy.Network.ListenIPVersion = "ipv6"
	rows := ProbeAssessmentListeners(context.Background(), config)
	if len(rows) != 2 || rows[0].Expected || !rows[0].Connected {
		t.Fatalf("unexpected IPv4 listener missed: %+v", rows)
	}
}
