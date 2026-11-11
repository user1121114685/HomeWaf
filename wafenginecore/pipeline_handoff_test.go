package wafenginecore

import (
	"HomeWaf/innerbean"
	"HomeWaf/wafpipeline"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPipelineHandoffStopsLegacySocketAndRestoresOnFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(strconv.FormatBool(fail), func(t *testing.T) {
			engine, front, hits := legacyBodyFixture(t)
			port := uint16(front.Listener.Addr().(*net.TCPAddr).Port)
			oldPorts := wafpipeline.Ports
			wafpipeline.Ports = wafpipeline.NewRegistry[wafpipeline.Snapshot](200)
			manager := &wafpipeline.ListenerManager{}
			t.Cleanup(func() {
				manager.StopAll()
				engine.StopAllProxyServer()
				wafpipeline.Ports = oldPorts
				wafpipeline.SetLegacyPortOwner(port, false)
			})
			address := "body.test:" + strconv.Itoa(int(port))
			var origin string
			engine.withWriteTable(func(rt *routingTable) {
				h := rt.HostTarget["body.test:5000"]
				origin = h.TargetHost
				rt.HostTarget[address] = h
				rt.HostCode["body-host"] = address
			})
			engine.ServerOnline.Set(int(port), innerbean.ServerRunTime{Port: int(port), ServerType: "http", IPVersion: "ipv4", Svr: front.Config})
			client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}, Timeout: 2 * time.Second}
			t.Cleanup(client.CloseIdleConnections)
			request := func(want string) {
				t.Helper()
				req, _ := http.NewRequest("POST", front.URL+"/plain", strings.NewReader(want))
				req.Host = address
				req.Header.Set("Content-Type", "application/octet-stream")
				res, err := client.Do(req)
				if err != nil {
					t.Fatal(err)
				}
				body, err := io.ReadAll(res.Body)
				res.Body.Close()
				if err != nil || res.StatusCode != 200 || string(body) != want {
					t.Fatalf("status=%d body=%q err=%v", res.StatusCode, body, err)
				}
			}
			request("legacy-before")
			wafpipeline.SetLegacyPortOwner(port, true)
			restore := engine.DetachPipelinePort(port, time.Second)
			if restore == nil {
				t.Fatal("legacy listener not detached")
			}
			if conn, err := net.DialTimeout("tcp", front.Listener.Addr().String(), 100*time.Millisecond); err == nil {
				conn.Close()
				t.Fatal("old listener remained open")
			}
			s := &wafpipeline.Snapshot{Config: wafpipeline.PortConfig{Schema: 1, Port: port, Protocol: "http", Upstream: origin, ListenEnabled: true, Policy: wafpipeline.DefaultPolicy()}}
			s.Config.Policy.Network.ListenIPVersion = "ipv4"
			s.Config.Policy.Gate.Enabled = false
			forward := &wafpipeline.Forwarder{}
			runtime := &wafpipeline.Runtime{ForwardPrepare: forward.Prepare, Forward: forward.Serve}
			if err := runtime.Prepare(s); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { wafpipeline.CloseRetiredProxy(s, nil) })
			manager.SetHandler(runtime.Serve)
			if err := wafpipeline.Ports.Publish(port, func() (*wafpipeline.Snapshot, error) { return s, nil }); err != nil {
				t.Fatal(err)
			}
			if fail {
				s.Config.Protocol = "https"
			}
			err := manager.Apply(nil, s)
			if fail {
				if err == nil {
					t.Fatal("invalid TLS certificate accepted")
				}
				if err := restore(); err != nil {
					t.Fatal(err)
				}
				if err := engine.WaitProxyReady(2 * time.Second); err != nil {
					t.Fatal(err)
				}
				request("legacy-restored")
			} else {
				if err != nil {
					t.Fatal(err)
				}
				engine.StartProxyServer(innerbean.ServerRunTime{Port: int(port), ServerType: "http", IPVersion: "ipv4", Status: 1})
				if _, ok := engine.ServerOnline.Get(int(port)); ok {
					t.Fatal("fenced legacy listener restarted")
				}
				request("native-after")
			}
			if hits.Load() != 2 {
				t.Fatalf("handoff duplicated or lost business requests: %d", hits.Load())
			}
		})
	}
}
