package wafpipeline

import (
	"math"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestGateFailureTimingBalancedAccounts(t *testing.T) {
	f := newGateFixture(t, nil, "form", "long-safe-pass-42")
	f.snap.Config.Policy.Gate.LoginPerMinute = 60
	if err := f.runtime.Prepare(f.snap); err != nil {
		t.Fatal(err)
	}
	const pairs = 20
	var known, unknown []float64
	for pair := 0; pair < pairs; pair++ {
		order := []string{"owner", "missing"}
		if pair%2 != 0 {
			order[0], order[1] = order[1], order[0]
		}
		var responseBody string
		for index, user := range order {
			started := time.Now()
			res, body := loginGate(t, f, user, "wrong-password")
			elapsed := float64(time.Since(started)) / float64(time.Millisecond)
			if res.StatusCode != 401 {
				t.Fatalf("timing sample returned %d", res.StatusCode)
			}
			if index == 0 {
				responseBody = body
			} else if body != responseBody {
				t.Fatal("account existence changed failure body")
			}
			if user == "owner" {
				known = append(known, elapsed)
			} else {
				unknown = append(unknown, elapsed)
			}
		}
	}
	stats := func(values []float64) (float64, float64) {
		var mean, v float64
		for _, n := range values {
			mean += n
		}
		mean /= float64(len(values))
		for _, n := range values {
			v += (n - mean) * (n - mean)
		}
		return mean, v / float64(len(values)-1)
	}
	km, kv := stats(known)
	um, uv := stats(unknown)
	paired := make([]float64, pairs)
	for i := range paired {
		paired[i] = known[i] - unknown[i]
	}
	delta, variance := stats(paired)
	pairedScore := 0.0
	if variance > 0 {
		pairedScore = math.Abs(delta) / math.Sqrt(variance/pairs)
	}
	se := math.Sqrt(kv/pairs + uv/pairs)
	score := 0.0
	if se > 0 {
		score = math.Abs(km-um) / se
	}
	t.Logf("GATE_TIMING pairs=%d known_mean_ms=%.3f unknown_mean_ms=%.3f welch_abs_t=%.3f paired_abs_t=%.3f", pairs, km, um, score, pairedScore)
	if score > 3 || pairedScore > 3 {
		t.Fatal("balanced timing samples show an account-dependent difference")
	}
	if f.hits.Load() != 0 {
		t.Fatal("invalid credentials reached upstream")
	}
}

func TestGateLoginReplacesAttackerChosenSession(t *testing.T) {
	f := newGateFixture(t, nil, "form", "long-safe-pass-42")
	csrf := csrfCookie(t, f)
	chosen := &http.Cookie{Name: gateCookieName(f.snap.Config.Port), Value: "attacker-chosen-session"}
	res, _ := gateRequest(t, f, "POST", f.snap.Config.Policy.Gate.Prefix+"/login", url.Values{"csrf": {csrf.Value}, "username": {"owner"}, "password": {"long-safe-pass-42"}, "next": {"/private"}}, nil, csrf, chosen)
	if res.StatusCode != 303 {
		t.Fatalf("login status=%d", res.StatusCode)
	}
	var issued *http.Cookie
	for _, cookie := range res.Cookies() {
		if cookie.Name == chosen.Name {
			issued = cookie
		}
	}
	if issued == nil || issued.Value == chosen.Value || issued.Value == "" {
		t.Fatal("login retained attacker session identifier")
	}
	denied, _ := gateRequest(t, f, "GET", "/private", nil, nil, chosen)
	if denied.StatusCode != 401 || f.hits.Load() != 0 {
		t.Fatal("attacker chosen session reached upstream")
	}
	allowed, _ := gateRequest(t, f, "GET", "/private", nil, nil, issued)
	if allowed.StatusCode != 200 || f.hits.Load() != 1 {
		t.Fatal("new signed session did not work")
	}
}
