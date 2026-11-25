package camouflage

import "net/netip"

// Preview classifies a synthetic connection without listeners, events or firewall writes.
func Preview(profile ProfileConfig, source netip.Addr, payload []byte, localCIDRs []string) Hit {
	probe, protocol, version, score, evidence := classify(profile.ProtocolProfile, payload, profile.Version)
	return Hit{ClientIP: source.String(), Port: profile.Port, Profile: profile.ProtocolProfile, Probe: probe, Protocol: protocol, Version: version, Score: score, Confidence: confidence(score), Evidence: evidence, LocalLAN: isLocalLANWithCIDRs(source, localCIDRs), Public: isPublic(source)}
}
