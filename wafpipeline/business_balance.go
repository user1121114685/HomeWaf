package wafpipeline

import (
	"HomeWaf/wafenginecore/loadbalance"
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type businessBalance struct {
	mu       sync.Mutex
	weighted *loadbalance.WeightRoundRobinBalance
	hash     *loadbalance.ConsistentHashBalance
	targets  []*ProxyTarget
}

func (h *UpstreamHealth) available() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.consecutiveFailures < 3 || time.Since(h.lastFailure) >= 5*time.Second
}
func (b *businessBalance) choose(ip string) *ProxyTarget {
	b.mu.Lock()
	defer b.mu.Unlock()
	healthy := func(_, id string) bool {
		index, err := strconv.Atoi(id)
		return err == nil && index >= 0 && index < len(b.targets) && b.targets[index].Health.available()
	}
	index := 0
	if b.hash != nil {
		value, err := b.hash.GetHealthy(ip, healthy)
		if err == nil {
			index, _ = strconv.Atoi(value)
		}
	} else if b.weighted != nil {
		if value, err := b.weighted.GetHealthy(healthy); err == nil {
			index = value
		}
	}
	if index < 0 || index >= len(b.targets) {
		index = 0
	}
	return b.targets[index]
}
func (f *Forwarder) prepareBalance(s *Snapshot, root, parent *ProxyTarget, route UpstreamRoute, dial func(context.Context, string, string) (net.Conn, error), idleSlots *int) error {
	if len(route.Backends) == 0 {
		return nil
	}
	balance := &businessBalance{}
	if route.Balance == "ip_hash" {
		balance.hash = loadbalance.NewConsistentHashBalance(nil, PortID(s.Config.Port))
	} else {
		balance.weighted = loadbalance.NewWeightRoundRobinBalance(PortID(s.Config.Port))
	}
	parent.balance = balance
	for _, backend := range route.Backends {
		if backend.Weight == 0 {
			continue
		}
		transport := root.transport.Clone()
		transport.MaxIdleConns, transport.MaxIdleConnsPerHost = 1, 1
		if *idleSlots > 0 {
			*idleSlots = *idleSlots - 1
		} else {
			transport.DisableKeepAlives = true
		}
		ip := backend.DialIP
		transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			if ip != "" {
				_, port, err := net.SplitHostPort(address)
				if err != nil {
					return nil, err
				}
				address = net.JoinHostPort(ip, port)
			}
			return dial(ctx, network, address)
		}
		target, err := f.prepareTarget(s, backend.Upstream, transport)
		if err != nil {
			transport.CloseIdleConnections()
			parent.closeIdle()
			return err
		}
		target.sharedHealth = &root.Health
		if route.StripPrefix {
			director := target.proxy.Director
			prefix := route.PathPrefix
			target.proxy.Director = func(r *http.Request) {
				u := *r.URL
				u.Path = "/" + strings.TrimPrefix(strings.TrimPrefix(u.Path, prefix), "/")
				u.RawPath = ""
				r.URL = &u
				director(r)
			}
		}
		if err = f.attachHeaders(s, target, route.HeaderProfile); err != nil {
			transport.CloseIdleConnections()
			parent.closeIdle()
			return err
		}
		index := len(balance.targets)
		balance.targets = append(balance.targets, target)
		if balance.hash != nil {
			_ = balance.hash.Add(strconv.Itoa(index), backend.Weight)
		} else {
			_ = balance.weighted.Add(index, backend.Weight)
		}
	}
	return nil
}
