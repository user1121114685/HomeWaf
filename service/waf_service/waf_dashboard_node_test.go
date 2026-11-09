package waf_service

import (
	"HomeWaf/model"
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
	located, err := decodeDashboardNode([]byte(`{"ip":"8.8.8.8","country":"US","city":"Los Angeles","region":"California","loc":"34.0559,-118.2666"}`), "ipinfo.io")
	if err != nil || !located.HasCoordinates || located.Latitude != 34.0559 || located.Longitude != -118.2666 {
		t.Fatalf("coordinate response: %+v %v", located, err)
	}
	for _, raw := range []string{
		`{"ip":"127.0.0.1","country":"CN","city":"上海"}`,
		`{"ip":"10.1.1.1","country":"CN","city":"上海"}`,
		`{"ip":"203.0.113.1","country":"CN","city":"上海"}`,
		`{"ip":"2001:db8::1","country":"CN","city":"上海"}`,
		`{"ip":"8.8.8.8","country":"US"}`,
		`{"error":{"message":"rate limit"}}`,
	} {
		if _, err := decodeDashboardNode([]byte(raw), "ipinfo.io"); err == nil {
			t.Fatalf("accepted invalid location: %s", raw)
		}
	}
	invalidCoords, err := decodeDashboardNode([]byte(`{"ip":"8.8.8.8","country":"US","city":"Los Angeles","loc":"NaN,300"}`), "ipinfo.io")
	if err != nil || invalidCoords.HasCoordinates {
		t.Fatalf("invalid coordinates must fall back to city: %+v %v", invalidCoords, err)
	}
}

type nodeRoundTrip func(*http.Request) (*http.Response, error)

func (f nodeRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDashboardNodeDomesticPriorityAndFallback(t *testing.T) {
	for _, domesticOK := range []bool{true, false} {
		calls := []string{}
		client := &http.Client{Transport: nodeRoundTrip(func(r *http.Request) (*http.Response, error) {
			calls = append(calls, r.URL.Host)
			if r.Header.Get("X-API-Key") != "" || r.Header.Get("X-Token") != "" {
				t.Fatal("credentials sent to provider")
			}
			if r.URL.Host == "myip.ipip.net" {
				if !domesticOK {
					return nil, errors.New("timeout")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ret":"ok","data":{"ip":"8.8.8.8","location":["美国","加利福尼亚州","洛杉矶"]}}`))}, nil
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ip":"8.8.8.8","country":"US","city":"Los Angeles","loc":"34,-118"}`))}, nil
		})}
		result, err := discoverDashboardNodeWithClient(context.Background(), client)
		if err != nil || calls[0] != "myip.ipip.net" {
			t.Fatalf("priority failed: %v %v", calls, err)
		}
		if domesticOK && (len(calls) != 1 || result.Source != "ipip.net") {
			t.Fatal("unnecessary external fallback")
		}
		if !domesticOK && (len(calls) != 2 || result.Source != "ipinfo.io") {
			t.Fatal("fallback not applied")
		}
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
