package wafpipeline

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"
)

type defenseConnectionKey struct{}

var defenseConnections sync.Map

var earlyDefenseSlots = make(chan struct{}, 16)
var defenseConnectionRejections atomic.Uint64

type defenseListener struct {
	net.Listener
	port uint16
}
type defenseConnection struct {
	net.Conn
	key  string
	once sync.Once
}

func connectionKey(c net.Conn) string { return c.LocalAddr().String() + "|" + c.RemoteAddr().String() }

func (l *defenseListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		if s, ok := Ports.Get(l.port); ok && Live.earlyDefenseDeny(s, conn.RemoteAddr()) {
			defenseConnectionRejections.Add(1)
			if s.Config.Policy.Defense.TCPAction == "drop" && Live.SilentDrop != nil {
				// Never block the listener's Accept loop on a system command.
				select {
				case earlyDefenseSlots <- struct{}{}:
					drop := Live.SilentDrop
					go func() {
						defer func() { <-earlyDefenseSlots }()
						req := (&http.Request{}).WithContext(context.WithValue(context.Background(), defenseConnectionKey{}, conn))
						_ = drop(req)
						resetConnection(conn)
					}()
					continue
				default:
				}
			}
			resetConnection(conn)
			continue
		}
		wrapped := &defenseConnection{Conn: conn, key: connectionKey(conn)}
		defenseConnections.Store(wrapped.key, wrapped)
		return wrapped, nil
	}
}

func (c *defenseConnection) Close() error {
	c.once.Do(func() { defenseConnections.CompareAndDelete(c.key, c) })
	return c.Conn.Close()
}

func withDefenseConnection(ctx context.Context, conn net.Conn) context.Context {
	if raw, ok := defenseConnections.Load(connectionKey(conn)); ok {
		return context.WithValue(ctx, defenseConnectionKey{}, raw.(*defenseConnection).Conn)
	}
	return ctx
}

func DefenseConnection(r *http.Request) net.Conn {
	conn, _ := r.Context().Value(defenseConnectionKey{}).(net.Conn)
	return conn
}

func resetConnection(conn net.Conn) {
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.SetLinger(0)
	}
	_ = conn.Close()
}

func AbortDefenseConnection(w http.ResponseWriter, r *http.Request) {
	if raw := DefenseConnection(r); raw != nil {
		resetConnection(raw)
	} else if conn, _, err := http.NewResponseController(w).Hijack(); err == nil {
		resetConnection(conn)
	}
	// In particular, never let net/http emit an implicit 200 after an abort.
	panic(http.ErrAbortHandler)
}

func (r *Runtime) earlyDefenseDeny(s *Snapshot, address net.Addr) bool {
	state := ActiveDefense(s.Config, time.Now().Unix())
	if state == nil || state.Mode != DefenseQuiet || s.Compiled == nil || len(s.Config.Policy.Network.TrustedProxies) != 0 {
		return false
	}
	ipPort, err := netip.ParseAddrPort(address.String())
	if err != nil {
		return false
	}
	ip := ipPort.Addr().Unmap()
	now := time.Now().Unix()
	if s.Compiled.Deny.Contains(ip.AsSlice(), now) {
		return true
	}
	if protectedAddress(ip) || s.Compiled.Trusted.Contains(ip.AsSlice(), now) || r.Protected != nil && r.Protected(ip) {
		return false
	}
	return r.State != nil && r.State.BanReason(PortID(s.Config.Port), ip) != ""
}
