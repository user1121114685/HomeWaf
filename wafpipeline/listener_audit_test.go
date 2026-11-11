package wafpipeline

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestMalformedConnectionNeverReachesHandlerAndIsCounted(t *testing.T) {
	old := Ports
	Ports = NewRegistry[Snapshot](200)
	t.Cleanup(func() { Ports = old })
	m := &ListenerManager{}
	sink := &safeSink{}
	m.SetEvents(sink)
	t.Cleanup(m.StopAll)
	var requests atomic.Int64
	m.SetHandler(func(s *Snapshot, w http.ResponseWriter, r *http.Request) { requests.Add(1); w.WriteHeader(400) })
	port := unusedPort(t)
	snap := &Snapshot{Config: PortConfig{Port: port, Protocol: "http", ListenEnabled: true, Policy: DefaultPolicy()}}
	snap.Config.Policy.Access.DrainSeconds = 1
	_ = Ports.Publish(port, func() (*Snapshot, error) { return snap, nil })
	if err := m.Start(snap); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"GET / HTTP/1.1\r\nHost: example.test\r\nContent-Length: 1\r\nContent-Length: 2\r\n\r\n", "GET / HTTP/1.1\r\nHost: example.test\r\nHost: other.test\r\n\r\n", "GET / HTTP/1.1\r\nHost: example.test\r\nBad Header: value\r\n\r\n"} {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
		if err != nil {
			t.Fatal(err)
		}
		conn.SetDeadline(time.Now().Add(3 * time.Second))
		_, _ = io.WriteString(conn, raw)
		line, err := bufio.NewReader(conn).ReadString('\n')
		conn.Close()
		if err != nil || !strings.Contains(line, "400") {
			t.Fatalf("parser response %q %v", line, err)
		}
	}
	if requests.Load() != 0 || m.ProtocolStats(port)["malformed_http1_responses"] != 3 {
		t.Fatalf("unobserved malformed requests: handler=%d stats=%v", requests.Load(), m.ProtocolStats(port))
	}
	if events := sink.snapshot(); len(events) != 3 {
		t.Fatal("parser rejection did not emit events")
	} else {
		for _, event := range events {
			if !event.Observe || event.RuleID != "http_parser" || event.PortID != PortID(port) || !event.ClientIP.IsLoopback() {
				t.Fatalf("unsafe parser attribution: %+v", event)
			}
		}
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/normal", port))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if requests.Load() != 1 || m.ProtocolStats(port)["malformed_http1_responses"] != 3 {
		t.Fatal("application 400 counted as parser failure")
	}
}
