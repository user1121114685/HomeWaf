package wafpipeline

import (
	"net/http"
	"path/filepath"
	"testing"
)

func TestCRSPathNormalizationIsHostIndependent(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{".ssh/id_rsa", ".ssh/id_rsa"},
		{"/sys/class", "/sys/class"},
		{"/etc/subuid", "/etc/subuid"},
		{"/safe/../etc/passwd", "/etc/passwd"},
		{`\etc\passwd`, "/etc/passwd"},
		{`C:\Windows\win.ini::$DATA`, "C:/Windows/win.ini"},
		{"/etc./passwd. ", "/etc/passwd"},
		{"/tmp/file:hidden/", "/tmp/file/"},
		{"../../etc/passwd", "../../etc/passwd"},
		{"/photos/holiday.jpg", "/photos/holiday.jpg"},
		{"/", "/"},
		{"", ""},
	} {
		t.Run(test.input, func(t *testing.T) {
			got, changed, err := normalizeCRSWindowsPath(test.input)
			if err != nil || got != test.want || changed != (got != test.input) {
				t.Fatalf("normalize %q = %q, changed=%v, err=%v; want %q", test.input, got, changed, err, test.want)
			}
		})
	}
	got, _, _ := normalizeCRSPath(`folder\name`)
	if got != `folder\name` {
		t.Fatal("POSIX normalization must preserve literal backslashes")
	}
}

func TestCRSFileAccessIsBlockedBeforeForwarding(t *testing.T) {
	fixture := newProxyFixture(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }, false)
	provider := &CRSProvider{Root: filepath.Join("..", "..", "..", "cmd", "homewaf", "exedata", "owasp")}
	fixture.runtime.RulePrepare = provider.Prepare
	fixture.runtime.RuleSteps = func(s *Snapshot) []Step { return []Step{provider.Step(s)} }
	fixture.update(t, func(s *Snapshot) {
		s.Config.Policy.RuleSet.OWASP = ModeBlock
		s.Config.Policy.RuleSet.BudgetMillis = 1000
	})
	headers := map[string]string{"User-Agent": "Mozilla/5.0", "Accept": "text/html"}
	for _, target := range []string{"/?q=.ssh/id_rsa", "/?q=/sys/class", "/?q=/etc/subuid", "/?q=%5Cetc%5Cpasswd"} {
		status, _, _ := proxyCall(t, fixture, "GET", target, "", headers)
		if status != http.StatusForbidden || fixture.requests.Load() != 0 {
			t.Fatalf("file access reached upstream: target=%s status=%d forwarded=%d", target, status, fixture.requests.Load())
		}
	}
	status, _, _ := proxyCall(t, fixture, "GET", "/photos?q=family+photo", "", headers)
	if status != http.StatusOK || fixture.requests.Load() != 1 {
		t.Fatal("benign traffic must reach upstream")
	}
	fixture.update(t, func(s *Snapshot) { s.Config.Policy.RuleSet.OWASP = ModeObserve })
	status, _, _ = proxyCall(t, fixture, "GET", "/?q=.ssh/id_rsa", "", headers)
	if status != http.StatusOK || fixture.requests.Load() != 2 {
		t.Fatal("observe mode must continue forwarding")
	}
}
