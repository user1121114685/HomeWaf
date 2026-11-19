// HomeWaf modification (2026-11-09): independent distribution; see NOTICE.

package utils

import (
	"reflect"
	"testing"

	"HomeWaf/model"
)

func TestExplicitHostListeners(t *testing.T) {
	host := model.Hosts{
		Port: 443, Ssl: 0, AutoJumpHTTPS: 1, BindMorePort: "80",
		PortListensJSON: `[{"port":8443,"proto":"http","ipv":"ipv6"},{"port":443,"proto":"https"}]`,
	}
	want := []HostListen{
		{Port: 443, Protocol: ListenProtoHTTPS, IPVersion: ListenIPVBoth, IsMain: true},
		{Port: 8443, Protocol: ListenProtoHTTP, IPVersion: ListenIPV6},
	}
	if got := ResolveHostListens(host); !reflect.DeepEqual(got, want) {
		t.Fatalf("declared listeners: got %+v, want %+v", got, want)
	}
	if ports := HostListenPorts(host); !reflect.DeepEqual(ports, []int{443, 8443}) {
		t.Fatalf("unexpected listener ports: %v", ports)
	}
	if got := HostMainProtocol(host); got != ListenProtoHTTPS {
		t.Fatalf("main protocol: %q", got)
	}
	host.GLOBAL_HOST = 1
	if len(ResolveHostListens(host)) != 0 {
		t.Fatal("global policy created a listener")
	}
}

func TestHostListenersRejectInvalidDeclarations(t *testing.T) {
	for _, raw := range []string{
		"", "{}", "[]", "{broken", `[{"port":8443,"proto":"https"}]`,
		`[{"port":80,"proto":"ftp"}]`, `[{"port":80,"proto":"http","ipv":"v6"}]`,
		`[{"port":80,"proto":"http","addr":"192.168.1.10"}]`,
		`[{"port":80,"proto":"http"},{"port":99999,"proto":"https"}]`,
		`[{"port":80,"proto":"http"},{"port":80,"proto":"https"}]`,
	} {
		host := model.Hosts{Port: 80, Ssl: 1, AutoJumpHTTPS: 1, BindMorePort: "443", PortListensJSON: raw}
		if got := ResolveHostListens(host); len(got) != 0 {
			t.Fatalf("invalid listener declaration %q accepted: %+v", raw, got)
		}
		if protocol := HostMainProtocol(host); protocol != "" {
			t.Fatalf("invalid listener declaration selected %q", protocol)
		}
	}
}
