package waf_service

import (
	"HomeWaf/internal/model"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestDashboardNodeProviders(t *testing.T) {
	city, err := decodeDashboardNode([]byte(`{"ret":"ok","data":{"ip":"8.8.8.8","location":["美国","加利福尼亚州","洛杉矶","","运营商"]}}`), "ipip.net")
	if err != nil || city.City != "洛杉矶" || city.HasCoordinates || city.PublicIP != "8.8.8.8" {
		t.Fatalf("city-only domestic response: %+v %v", city, err)
	}
	plain, err := decodeDashboardNode([]byte("当前 IP：8.8.8.8  来自于：美国 加利福尼亚州 洛杉矶  运营商"), "ipip.net")
	if err != nil || plain.City != "洛杉矶" {
		t.Fatalf("plain response: %+v %v", plain, err)
	}
	for _, raw := range []string{
		`{"ret":"ok","data":{"ip":"127.0.0.1","location":["CN","region","city"]}}`,
		`{"ret":"ok","data":{"ip":"10.1.1.1","location":["CN","region","city"]}}`,
		`{"ret":"ok","data":{"ip":"203.0.113.1","location":["CN","region","city"]}}`,
		`{"ret":"ok","data":{"ip":"2001:db8::1","location":["CN","region","city"]}}`,
		`{"ret":"ok","data":{"ip":"8.8.8.8","location":["US"]}}`,
		`{"ret":"error"}`,
	} {
		if _, err := decodeDashboardNode([]byte(raw), "ipip.net"); err == nil {
			t.Fatalf("accepted invalid location: %s", raw)
		}
	}

}

type nodeRoundTrip func(*http.Request) (*http.Response, error)

func (f nodeRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDashboardNodeDomesticPriorityAndFallback(t *testing.T) {
	for _, firstOK := range []bool{true, false} {
		calls := []string{}
		client := &http.Client{Transport: nodeRoundTrip(func(r *http.Request) (*http.Response, error) {
			calls = append(calls, r.URL.String())
			if r.URL.Host != "myip.ipip.net" {
				t.Fatal("overseas fallback changes the observed exit")
			}
			if r.Header.Get("X-Token") != "" || r.Header.Get("X-API-Key") != "" {
				t.Fatal("credentials sent to provider")
			}
			if r.URL.Path == "/json" && !firstOK {
				return nil, errors.New("temporary JSON failure")
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ret":"ok","data":{"ip":"8.8.8.8","location":["CN","region","city"]}}`))}, nil
		})}
		result, err := discoverDashboardNodeWithClient(context.Background(), client)
		if err != nil || result.Source != "ipip.net" {
			t.Fatalf("domestic lookup failed: %v", err)
		}
		if firstOK && len(calls) != 1 || !firstOK && (len(calls) != 2 || calls[1] != "https://myip.ipip.net/") {
			t.Fatalf("unexpected fallback: %v", calls)
		}
	}
	calls := 0
	client := &http.Client{Transport: nodeRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host != "myip.ipip.net" {
			t.Fatal("overseas fallback")
		}
		return nil, errors.New("offline")
	})}
	if _, err := discoverDashboardNodeWithClient(context.Background(), client); err == nil || calls != 2 {
		t.Fatal("failed domestic lookup fabricated a position")
	}
	transport, ok := dashboardNodeClient.Transport.(*http.Transport)
	if !ok || transport.Proxy != nil {
		t.Fatal("node discovery inherited proxy settings")
	}
	if dashboardNodeClient.CheckRedirect(nil, nil) != http.ErrUseLastResponse {
		t.Fatal("node lookup followed external redirect")
	}
}

func TestDashboardNodeNonblockingFailureHasNoPosition(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	c := &dashboardNodeCache{lookup: func(ctx context.Context) (model.DashboardTarget, error) {
		close(started)
		<-release
		return model.DashboardTarget{}, errors.New("offline")
	}}
	first := c.snapshot()
	if first.Status != "locating" || first.HasCoordinates || first.PublicIP != "" {
		t.Fatalf("fabricated pending location: %+v", first)
	}
	<-started
	for range 5 {
		if c.snapshot().Status != "locating" {
			t.Fatal("did not coalesce requests")
		}
	}
	close(release)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		data := c.snapshot()
		if data.Status == "unavailable" {
			if data.HasCoordinates || data.PublicIP != "" {
				t.Fatalf("fabricated failed position: %+v", data)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("discovery did not finish")
}

func TestDashboardNodeRefreshKeepsLastKnownLocation(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	lastKnown := model.DashboardTarget{
		Mode: "auto", Label: "HomeWaf 防护节点", Status: "ready",
		PublicIP: "8.8.8.8", Country: "美国", Region: "加利福尼亚州", City: "洛杉矶",
		Latitude: 34.0559, Longitude: -118.2666, HasCoordinates: true, CheckedAt: 123,
	}
	c := &dashboardNodeCache{
		data: lastKnown,
		lookup: func(ctx context.Context) (model.DashboardTarget, error) {
			close(started)
			<-release
			return model.DashboardTarget{}, errors.New("offline")
		},
	}
	if current := c.snapshot(); current.PublicIP != lastKnown.PublicIP {
		t.Fatalf("refresh removed the last known location: %+v", current)
	}
	<-started
	if current := c.snapshot(); current.Longitude != lastKnown.Longitude {
		t.Fatalf("refresh changed the last known location: %+v", current)
	}
	close(release)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		current := c.snapshot()
		if current.Status == "stale" {
			if current.PublicIP != lastKnown.PublicIP || current.Longitude != lastKnown.Longitude || current.CheckedAt != lastKnown.CheckedAt {
				t.Fatalf("stale location was not retained: %+v", current)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("failed refresh did not retain the last known location")
}
