package wafpipeline

import (
	"bytes"
	"context"
	"log"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type protocolConnectionKey struct{}
type protocolConnection struct{ active, handled, confirmed atomic.Bool }
type protocolAudit struct {
	port                                                 uint16
	sink                                                 EventSink
	connections                                          sync.Map
	malformed, preHandlerClosed, tlsErrors, serverErrors atomic.Uint64
}

func (a *protocolAudit) attach(server *http.Server) {
	server.ConnContext = func(ctx context.Context, conn net.Conn) context.Context {
		state := &protocolConnection{}
		if observed, ok := conn.(*protocolConn); ok {
			state = observed.state
		}
		a.connections.Store(conn, state)
		return context.WithValue(ctx, protocolConnectionKey{}, state)
	}
	server.ConnState = func(conn net.Conn, state http.ConnState) {
		value, ok := a.connections.Load(conn)
		if !ok {
			return
		}
		c := value.(*protocolConnection)
		switch state {
		case http.StateActive:
			c.active.Store(true)
			c.handled.Store(false)
		case http.StateClosed:
			if c.active.Load() && !c.handled.Load() {
				a.preHandlerClosed.Add(1)
			}
			a.connections.Delete(conn)
		case http.StateHijacked:
			a.connections.Delete(conn)
		}
	}
	server.ErrorLog = log.New(a, "", 0)
}
func (a *protocolAudit) Write(p []byte) (int, error) {
	if strings.Contains(string(p), "TLS handshake error") {
		a.tlsErrors.Add(1)
	} else {
		a.serverErrors.Add(1)
	}
	return len(p), nil
}
func (a *protocolAudit) stats() map[string]uint64 {
	return map[string]uint64{"malformed_http1_responses": a.malformed.Load(), "pre_handler_closed": a.preHandlerClosed.Load(), "tls_errors": a.tlsErrors.Load(), "server_errors": a.serverErrors.Load()}
}

type protocolListener struct {
	net.Listener
	audit *protocolAudit
}

func (l *protocolListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &protocolConn{Conn: conn, audit: l.audit, state: &protocolConnection{}}, nil
}

type protocolConn struct {
	net.Conn
	audit *protocolAudit
	state *protocolConnection
}

func (c *protocolConn) Write(p []byte) (int, error) {
	if !c.state.handled.Load() && !c.state.confirmed.Load() && (bytes.HasPrefix(p, []byte("HTTP/1.1 400 ")) || bytes.HasPrefix(p, []byte("HTTP/1.1 431 "))) {
		if c.state.confirmed.CompareAndSwap(false, true) {
			c.audit.malformed.Add(1)
			if c.audit.sink != nil {
				peer, _, err := net.SplitHostPort(c.RemoteAddr().String())
				ip, parseErr := netip.ParseAddr(peer)
				if err == nil && parseErr == nil {
					status := 400
					if bytes.HasPrefix(p, []byte("HTTP/1.1 431 ")) {
						status = 431
					}
					c.audit.sink.Submit(HitEvent{Time: time.Now(), PortID: PortID(c.audit.port), ClientIP: ip.Unmap(), Category: "malformed", RuleID: "http_parser", Severity: High, Observe: true, Evidence: "HTTP parser rejected request before handler; TCP peer only", ReqID: randomToken(), Status: status})
				}
			}
		}
	}
	return c.Conn.Write(p)
}
func markProtocolHandler(r *http.Request) {
	if c, ok := r.Context().Value(protocolConnectionKey{}).(*protocolConnection); ok {
		c.handled.Store(true)
	}
}
func (m *ListenerManager) ProtocolStats(port uint16) map[string]uint64 {
	m.mu.Lock()
	p := m.servers[port]
	m.mu.Unlock()
	if p == nil || p.audit == nil {
		return map[string]uint64{}
	}
	stats := p.audit.stats()
	stats["connections"] = uint64(p.tracker.Count())
	stats["global_connections"] = uint64(activePipelineConnections.Load())
	stats["global_connection_rejections"] = rejectedPipelineConnections.Load()
	return stats
}
