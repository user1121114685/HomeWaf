package wafowasp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
