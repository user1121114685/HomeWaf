package wafpipeline

import (
	"HomeWaf/wafai"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

type pipelineAIStub struct {
	calls     atomic.Int64
	bodyBytes atomic.Int64
	unloaded  atomic.Bool
}

func (a *pipelineAIStub) PredictRequest(method, path, query, body, ua string) wafai.PredictResult {
	a.calls.Add(1)
	a.bodyBytes.Store(int64(len(body)))
	return wafai.PredictResult{Loaded: !a.unloaded.Load(), Score: 0.95, ObserveThreshold: 0.5, BlockThreshold: 0.8}
}
func TestLocalAIModesGateOrderingAndBoundedInspection(t *testing.T) {
	f := newProxyFixture(t, func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			w.WriteHeader(500)
			return
		}
		_, _ = w.Write(body)
	}, false)
	model := &pipelineAIStub{}
	f.runtime.AI = model
	f.update(t, func(s *Snapshot) {
		s.Config.Policy.RuleSet.AI = ModeObserve
		s.Config.Policy.RuleSet.BudgetMillis = 1000
	})
	wire := strings.Repeat("family-content", 5000)
	code, body, _ := proxyCall(t, f, "POST", "/notes", wire, nil)
	if code != 200 || !bytes.Equal(body, []byte(wire)) || model.bodyBytes.Load() != 32768 || f.requests.Load() != 1 {
		t.Fatalf("AI inspection damaged or over-read business stream: status=%d body=%d/%d inspected=%d calls=%d upstream=%d", code, len(body), len(wire), model.bodyBytes.Load(), model.calls.Load(), f.requests.Load())
	}
	for _, event := range f.sink.snapshot() {
		if event.Category == "ai" && !event.Observe {
			t.Fatal("observation mode emitted punishment")
		}
	}
	f.update(t, func(s *Snapshot) { s.Config.Policy.RuleSet.AI = ModeBlock })
	code, _, _ = proxyCall(t, f, "GET", "/blocked", "", nil)
	if code != 403 || f.requests.Load() != 1 {
		t.Fatal("block mode reached origin")
	}
	model.unloaded.Store(true)
	code, _, _ = proxyCall(t, f, "GET", "/unavailable", "", nil)
	if code != 200 || f.requests.Load() != 2 {
		t.Fatal("unavailable AI model did not fail open")
	}
	model.unloaded.Store(false)
	f.update(t, func(s *Snapshot) { s.Config.Policy.Gate.Enabled = true })
	calls := model.calls.Load()
	code, _, _ = proxyCall(t, f, "GET", "/private", "", nil)
	if code != 401 || model.calls.Load() != calls || f.requests.Load() != 2 {
		t.Fatal("AI ran before authentication")
	}
}
func TestPipelineAIReusesExistingLocalModel(t *testing.T) {
	model := wafai.NewDetector()
	if _, err := model.LoadFromFile(filepath.Join("..", "wafai", "testdata", "sample_model.swai")); err != nil {
		t.Fatal(err)
	}
	runtime := &Runtime{AI: model}
	policy := DefaultPolicy()
	policy.RuleSet.AI = ModeBlock
	policy.RuleSet.BudgetMillis = 1000
	s := &Snapshot{Config: PortConfig{Schema: 1, Port: 5300, Protocol: "http", Upstream: "http://unused.invalid", Policy: policy}}
	for _, query := range []string{"page=1&sort=asc", "id=1' or '1'='1 union select null,null--"} {
		req := httptest.NewRequest("GET", "http://family.test/index.php", nil)
		req.URL.RawQuery = query
		req.Header.Set("User-Agent", "sqlmap/1.5")
		prediction := model.PredictRequest("GET", "/index.php", query, "", "sqlmap/1.5")
		result, err := runtime.Explain(s, req, netip.MustParseAddr("198.51.100.20"))
		if err != nil {
			t.Fatal(err)
		}
		blocked := prediction.Loaded && prediction.Score >= prediction.ObserveThreshold && prediction.Score >= prediction.BlockThreshold
		if (result.Verdict.Action == Deny) != blocked {
			t.Fatalf("native model result changed: prediction=%+v verdict=%+v", prediction, result.Verdict)
		}
	}
}
