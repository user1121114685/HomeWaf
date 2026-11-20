package wafpipeline

import (
	"testing"
	"time"
)

func TestGateBulkRevocationScopeAndTokenValidity(t *testing.T) {
	store := NewGateStore()
	now := time.Now().Unix()
	for _, port := range []string{"port-a", "port-b"} {
		store.ReplaceAccounts(port, []GateAccount{{PortID: port, Username: "one", Version: "v1", Enabled: true}, {PortID: port, Username: "two", Version: "v1", Enabled: true}})
		for _, user := range []string{"one", "two"} {
			if err := store.Add(GateSessionRecord{ID: port + user, PortID: port, Username: user, TokenHash: port + user, Version: "v1", CreatedAt: now, LastSeen: now, ExpiresAt: now + 3600}, 0); err != nil {
				t.Fatal(err)
			}
		}
	}
	if store.RevokeScope("", "") != 0 {
		t.Fatal("empty port must not mean all ports")
	}
	if store.RevokeScope("port-a", "one") != 1 {
		t.Fatal("account scope")
	}
	if _, valid := store.Valid("port-aone", "port-a", "", GateConfig{}, false); valid {
		t.Fatal("revoked token still works")
	}
	if _, valid := store.Valid("port-atwo", "port-a", "", GateConfig{}, false); !valid {
		t.Fatal("other account revoked")
	}
	if store.RevokeScope("port-a", "") != 1 {
		t.Fatal("service scope")
	}
	if len(store.Sessions("port-b")) != 2 {
		t.Fatal("cross-port revocation")
	}
	var persisted []GateSessionRecord
	store.Persist = func(rows []GateSessionRecord) error { persisted = rows; return nil }
	if err := store.Flush(); err != nil {
		t.Fatal(err)
	}
	if len(persisted) != 2 || persisted[0].PortID != "port-b" || persisted[1].PortID != "port-b" {
		t.Fatal("revocation was not persisted")
	}
}
