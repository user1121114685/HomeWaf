package wafowasp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corazawaf/coraza/v3/experimental/plugins/plugintypes"
)

func TestTuningPreservesIndependentCRSVariables(t *testing.T) {
	root := t.TempDir()
	store := NewOverrideStore(filepath.Join(root, "overrides"), filepath.Join(root, "samwaf"))
	if err := store.EnsureDirAndDefaults(); err != nil {
		t.Fatal(err)
	}
	if err := store.SetCRSVar("allowed_methods", "GET POST"); err != nil {
		t.Fatal(err)
	}
	tuning := DefaultTuning()
	tuning.InboundThreshold = 9
	tuning.EnforceBodyProcessor = 1
	if err := store.SetTuning(tuning); err != nil {
		t.Fatal(err)
	}
	// Reopen the store to verify persistence, not just an in-memory snapshot.
	reopened := NewOverrideStore(store.Dir(), filepath.Join(root, "samwaf"))
	actual, err := reopened.GetTuning()
	if err != nil {
		t.Fatal(err)
	}
	if actual.CustomVars["allowed_methods"] != "GET POST" || actual.InboundThreshold != 9 || actual.EnforceBodyProcessor != 1 {
		t.Fatalf("tuning lost independently maintained configuration: %#v", actual)
	}
	generated, err := os.ReadFile(filepath.Join(store.Dir(), OverrideTuningFile))
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"tx.allowed_methods=GET POST", "tx.enforce_bodyproc_urlencoded=1"} {
		if !strings.Contains(string(generated), value) {
			t.Errorf("runtime configuration missing %q", value)
		}
	}
	if err := reopened.DeleteCRSVar("allowed_methods"); err != nil {
		t.Fatal(err)
	}
	actual, err = reopened.GetTuning()
	if err != nil || len(actual.CustomVars) != 0 {
		t.Fatalf("explicit variable deletion failed: %#v, %v", actual.CustomVars, err)
	}
	if err := reopened.SetCRSVar("allowed_methods", "GET"); err != nil {
		t.Fatal(err)
	}
	tuning.CustomVars = map[string]string{}
	if err := reopened.SetTuning(tuning); err != nil {
		t.Fatal(err)
	}
	actual, err = reopened.GetTuning()
	if err != nil || len(actual.CustomVars) != 0 {
		t.Fatalf("explicit empty-map replacement failed: %#v, %v", actual.CustomVars, err)
	}
}

func TestBaseConfigReloadPreservesUserOverrideAndPersists(t *testing.T) {
	dir := t.TempDir()
	root := minimalOwaspRoot(t, dir)
	if err := os.WriteFile(filepath.Join(root, "coreruleset", "rules", "old-rule.conf"), []byte("SecAction \"id:9900001,phase:1,pass,nolog\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	m := NewOwaspManager(dir)
	if m.Current() == nil {
		t.Fatal("fixture engine did not initialize")
	}
	tuning := DefaultTuning()
	tuning.InboundThreshold = 11
	if err := m.ApplyTuning(tuning); err != nil {
		t.Fatal(err)
	}
	before := m.Current()
	base := DefaultBaseConfig()
	base.InboundThreshold = 8
	if err := m.ApplyBaseConfig(base); err != nil {
		t.Fatal(err)
	}
	if m.Current() == before {
		t.Fatal("baseline did not publish a reloaded engine")
	}
	tx := m.Current().WAF.NewTransaction()
	defer tx.Close()
	tx.ProcessURI("/", "GET", "HTTP/1.1")
	tx.AddRequestHeader("Host", "fixture.test")
	tx.ProcessRequestHeaders()
	values := tx.(plugintypes.TransactionState).Variables().TX().Get("inbound_anomaly_score_threshold")
	if len(values) == 0 || values[0] != "11" {
		t.Fatalf("user override did not win at runtime: %v", values)
	}
	store := NewOverrideStore(m.OverridesDir(), filepath.Join(root, "samwaf"))
	actual, err := store.GetBaseConfig()
	if err != nil || actual.InboundThreshold != 8 {
		t.Fatalf("baseline not durable: %+v %v", actual, err)
	}
	user, err := store.GetTuning()
	if err != nil || user.InboundThreshold != 11 {
		t.Fatalf("baseline overwrote user settings: %+v %v", user, err)
	}
}
