package wafpipeline

import "sync"

// Administrative ownership is separate from whether a listener is running.
// A stopped/deleted migrated port must never reopen through the legacy loader.
var legacyPortOwners = struct {
	sync.RWMutex
	ports map[uint16]bool
}{ports: map[uint16]bool{}}

func OwnsLegacyPort(port uint16) bool {
	legacyPortOwners.RLock()
	defer legacyPortOwners.RUnlock()
	return legacyPortOwners.ports[port]
}

func SetLegacyPortOwner(port uint16, owned bool) {
	legacyPortOwners.Lock()
	defer legacyPortOwners.Unlock()
	if owned {
		legacyPortOwners.ports[port] = true
	} else {
		delete(legacyPortOwners.ports, port)
	}
}

func ReplaceLegacyPortOwners(ports []uint16) {
	next := make(map[uint16]bool, len(ports))
	for _, port := range ports {
		next[port] = true
	}
	legacyPortOwners.Lock()
	legacyPortOwners.ports = next
	legacyPortOwners.Unlock()
}
