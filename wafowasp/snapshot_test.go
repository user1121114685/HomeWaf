package wafowasp

import "testing"

func TestActivationPublishesNewImmutableInstance(t *testing.T) {
	m := &OwaspManager{}
	old := &WafOWASP{IsActive: true}
	m.current.Store(old)
	m.active.Store(true)
	m.SetActive(false)
	if !old.IsActive || m.Current() == old || m.Current().IsActive {
		t.Fatal("activation mutated a published snapshot")
	}
}
