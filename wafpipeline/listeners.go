package wafpipeline

import (
	"HomeWaf/wafnet"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type PortHandler func(*Snapshot, http.ResponseWriter, *http.Request)
type handlerHolder struct{ handler PortHandler }
type portListener struct {
	tls              atomic.Pointer[tls.Config]
	server           *http.Server
	tracker          *wafnet.ConnTracker
	ready            *wafnet.Readiness
	drain            time.Duration
	audit            *protocolAudit
	accessTimer      *time.Timer
	accessGeneration uint64
	serveError       atomic.Value
}
type ListenerManager struct {
	mu      sync.Mutex
	closing bool
	servers map[uint16]*portListener
	handler atomic.Pointer[handlerHolder]
	events  EventSink
}

var Listeners = &ListenerManager{servers: make(map[uint16]*portListener)}

func (m *ListenerManager) SetHandler(h PortHandler) { m.handler.Store(&handlerHolder{h}) }
func (m *ListenerManager) SetEvents(sink EventSink) { m.mu.Lock(); m.events = sink; m.mu.Unlock() }
func (m *ListenerManager) Apply(old, next *Snapshot) error {
	if next == nil {
		if old != nil {
			return m.Stop(old.Config.Port)
		}
		return nil
	}
	if !next.Config.ListenEnabled {
		return m.Stop(next.Config.Port)
	}
	if err := m.Start(next); err != nil {
		return err
	}
	m.updateAccessDrain(old, next)
	return nil
}

func (m *ListenerManager) updateAccessDrain(old, next *Snapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := m.servers[next.Config.Port]
	if p == nil {
		return
	}
	if next.Config.Policy.Access.Enabled {
		p.accessGeneration++
		if p.accessTimer != nil {
			p.accessTimer.Stop()
			p.accessTimer = nil
		}
		return
	}
	if old == nil || !old.Config.Policy.Access.Enabled {
		return
	}
	p.accessGeneration++
	generation := p.accessGeneration
	if p.accessTimer != nil {
		p.accessTimer.Stop()
	}
	p.accessTimer = time.AfterFunc(time.Duration(next.Config.Policy.Access.DrainSeconds)*time.Second, func() {
		m.mu.Lock()
		current := m.servers[next.Config.Port]
		valid := current == p && p.accessGeneration == generation
		m.mu.Unlock()
		if !valid {
			return
		}
		if snapshot, ok := Ports.Get(next.Config.Port); !ok || snapshot.Config.Policy.Access.Enabled {
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_ = p.tracker.Drain(ctx)
	})
}
func (m *ListenerManager) Start(s *Snapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closing {
		return errors.New("监听管理器已进入退出排空，不能重新启动端口")
	}
	if m.servers == nil {
		m.servers = make(map[uint16]*portListener)
	}
	port := s.Config.Port
	var certificates []tls.Certificate
	if s.Config.Protocol == "https" {
		certificate, err := tls.X509KeyPair([]byte(s.Secrets.TLSCert), []byte(s.Secrets.TLSKey))
		if err != nil {
			return errors.New("证书或私钥不合法")
		}
		certificates = append(certificates, certificate)
		if len(s.Secrets.AdditionalTLS) > 100 {
			return errors.New("端口证书数量超过上限")
		}
		for _, pair := range s.Secrets.AdditionalTLS {
			certificate, err := tls.X509KeyPair([]byte(pair.Certificate), []byte(pair.PrivateKey))
			if err != nil {
				return errors.New("附加证书或私钥不合法")
			}
			certificates = append(certificates, certificate)
		}
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, Certificates: certificates, NextProtos: []string{"h2", "http/1.1"}}
	if s.Config.Policy.Network.DisableHTTP2 {
		tlsConfig.NextProtos = []string{"http/1.1"}
	}
	if current, ok := m.servers[port]; ok {
		if s.Config.Protocol == "https" {
			current.tls.Store(tlsConfig)
		}
		return nil
	}
	network := "tcp"
	switch s.Config.Policy.Network.ListenIPVersion {
	case "ipv4":
		network = "tcp4"
	case "ipv6":
		network = "tcp6"
	}
	ln, err := wafnet.ReusePortTCPListenNetwork(network, ":"+strconv.Itoa(int(port)))
	if err != nil {
		return fmt.Errorf("端口 %d 无法监听: %w", port, err)
	}
	tracker, ready := &wafnet.ConnTracker{}, &wafnet.Readiness{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		markProtocolHandler(r)
		snap, ok := Ports.Get(port)
		if !ok || !snap.Config.ListenEnabled {
			http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
			return
		}
		h := m.handler.Load()
		if h == nil || h.handler == nil {
			http.Error(w, "Port policy is not ready", http.StatusServiceUnavailable)
			return
		}
		h.handler(snap, w, r)
	})
	server := &http.Server{Handler: handler, ReadHeaderTimeout: time.Duration(s.Config.Policy.Network.ReadHeaderSeconds) * time.Second, MaxHeaderBytes: s.Config.Policy.Network.MaxHeaderBytes, IdleTimeout: 90 * time.Second}
	audit := &protocolAudit{port: port, sink: m.events}
	audit.attach(server)
	runtime := &portListener{server: server, tracker: tracker, ready: ready, drain: time.Duration(s.Config.Policy.Access.DrainSeconds) * time.Second, audit: audit}
	if s.Config.Protocol == "https" {
		runtime.tls.Store(tlsConfig)
		server.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: certificates, GetConfigForClient: func(*tls.ClientHelloInfo) (*tls.Config, error) { return runtime.tls.Load(), nil }}
		if s.Config.Policy.Network.DisableHTTP2 {
			server.TLSNextProto = map[string]func(*http.Server, *tls.Conn, http.Handler){}
		}
	}
	m.servers[port] = runtime
	go func() {
		wrapped := wafnet.WithReadiness(tracker.Listener(limitPipelineListener(ln, s.Config.Policy.Network.MaxInFlight)), ready)
		var err error
		if s.Config.Protocol == "https" {
			err = server.ServeTLS(wrapped, "", "")
		} else {
			err = server.Serve(&protocolListener{Listener: wrapped, audit: audit})
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			runtime.serveError.Store("监听异常终止，请检查日志并重新启用")
			ready.Fail(err)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := ready.Wait(ctx); err != nil {
		delete(m.servers, port)
		_ = server.Close()
		return err
	}
	return nil
}
func (m *ListenerManager) Stop(port uint16) error {
	m.mu.Lock()
	p := m.servers[port]
	if p != nil {
		p.accessGeneration++
		if p.accessTimer != nil {
			p.accessTimer.Stop()
		}
	}
	delete(m.servers, port)
	m.mu.Unlock()
	if p == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), p.drain)
	defer cancel()
	err := p.server.Shutdown(ctx)
	_ = p.tracker.Drain(ctx)
	if err != nil {
		_ = p.server.Close()
	}
	return nil
}
func (m *ListenerManager) StopAll() {
	m.mu.Lock()
	m.closing = true
	ports := make([]uint16, 0, len(m.servers))
	for p := range m.servers {
		ports = append(ports, p)
	}
	m.mu.Unlock()
	var wg sync.WaitGroup
	for _, p := range ports {
		wg.Add(1)
		go func(port uint16) { defer wg.Done(); _ = m.Stop(port) }(p)
	}
	wg.Wait()
	for _, port := range ports {
		if snapshot, ok := Ports.Get(port); ok {
			CloseRetiredProxy(snapshot, nil)
		}
	}
}
func (m *ListenerManager) StartAll() error {
	var failures []error
	for _, port := range Ports.Ports() {
		s, _ := Ports.Get(port)
		if s != nil && s.Config.ListenEnabled {
			if err := m.Start(s); err != nil {
				m.mu.Lock()
				running := m.servers[port] != nil
				m.mu.Unlock()
				if !running {
					CloseRetiredProxy(s, nil)
				}
				failures = append(failures, err)
			}
		}
	}
	return errors.Join(failures...)
}
func (m *ListenerManager) WaitReady(ctx context.Context) error {
	m.mu.Lock()
	waiters := make([]*wafnet.Readiness, 0, len(m.servers))
	for _, p := range m.servers {
		waiters = append(waiters, p.ready)
	}
	m.mu.Unlock()
	for _, r := range waiters {
		if err := r.Wait(ctx); err != nil {
			return err
		}
	}
	return nil
}
func (m *ListenerManager) Count() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	var n int64
	for _, p := range m.servers {
		n += p.tracker.Count()
	}
	return n
}

func (m *ListenerManager) Status(port uint16) map[string]any {
	m.mu.Lock()
	p := m.servers[port]
	m.mu.Unlock()
	if p == nil {
		return map[string]any{"running": false, "error": ""}
	}
	if err, ok := p.serveError.Load().(string); ok {
		return map[string]any{"running": false, "error": err}
	}
	return map[string]any{"running": true, "error": ""}
}
