package waf_service

import (
	"HomeWaf/common/zlog"
	"HomeWaf/wafpipeline"
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

type tamperCapture struct {
	Port     uint16
	Revision int64
	Rule     wafpipeline.TamperRule
	Baseline wafpipeline.TamperBaseline
}
type PipelineTamperService struct {
	owner           *WafPipelineService
	queue           chan tamperCapture
	mu              sync.Mutex
	pending         map[uint16]bool
	running         atomic.Bool
	dropped, failed atomic.Uint64
	Started         chan struct{}
}

var WafPipelineTamperServiceApp = NewPipelineTamperService(WafPipelineServiceApp)

func (s *PipelineTamperService) Stats() map[string]uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return map[string]uint64{"queued": uint64(len(s.queue)), "capacity": uint64(cap(s.queue)), "pending_ports": uint64(len(s.pending)), "dropped": s.dropped.Load(), "failed": s.failed.Load()}
}

func NewPipelineTamperService(owner *WafPipelineService) *PipelineTamperService {
	return &PipelineTamperService{owner: owner, queue: make(chan tamperCapture, 32), pending: map[uint16]bool{}, Started: make(chan struct{})}
}
func (s *PipelineTamperService) Capture(snapshot *wafpipeline.Snapshot, rule wafpipeline.TamperRule, body []byte, contentType, charset string) bool {
	if !s.running.Load() || snapshot.Preview || len(body) > 1<<20 {
		s.dropped.Add(1)
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running.Load() || s.pending[snapshot.Config.Port] || len(s.queue) == cap(s.queue) {
		s.dropped.Add(1)
		return false
	}
	entry := tamperCapture{snapshot.Config.Port, snapshot.Config.Revision, rule, wafpipeline.NewTamperBaseline(rule, body, contentType, charset)}
	select {
	case s.queue <- entry:
		s.pending[entry.Port] = true
		return true
	default:
		return false
	}
}
func (s *PipelineTamperService) apply(entry tamperCapture) error {
	current, err := s.owner.Get(entry.Port)
	if err != nil {
		return err
	}
	if current.Revision != entry.Revision {
		return errors.New("配置版本已变化，丢弃旧防篡改采样")
	}
	found := false
	for _, rule := range current.Policy.Network.Response.Tamper {
		if rule.ID == entry.Rule.ID && wafpipeline.TamperBinding(rule) == entry.Baseline.Binding {
			found = true
			break
		}
	}
	if !found {
		return errors.New("防篡改规则已变化")
	}
	_, err = s.owner.Save(PortSaveRequest{Config: current, TamperUpdates: map[string]wafpipeline.TamperBaseline{entry.Rule.ID: entry.Baseline}, Operator: "tamper_capture"})
	return err
}
func (s *PipelineTamperService) process(entry tamperCapture) {
	if err := s.apply(entry); err != nil {
		s.failed.Add(1)
		zlog.Warn("防篡改基线采样未保存", err.Error())
	}
	s.mu.Lock()
	delete(s.pending, entry.Port)
	s.mu.Unlock()
}
func (s *PipelineTamperService) Run(ctx context.Context) {
	s.running.Store(true)
	close(s.Started)
	defer s.running.Store(false)
	for {
		select {
		case entry := <-s.queue:
			s.process(entry)
		case <-ctx.Done():
			s.mu.Lock()
			s.running.Store(false)
			s.mu.Unlock()
			for {
				select {
				case entry := <-s.queue:
					s.process(entry)
				default:
					return
				}
			}
		}
	}
}
