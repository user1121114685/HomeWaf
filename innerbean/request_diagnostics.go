package innerbean

// RequestDiagnostics is a read-only projection of retained pipeline events.
// Request values and arbitrary detector evidence are deliberately excluded.
type RequestDiagnostics struct {
	State         string          `json:"state"`
	BodyTruncated bool            `json:"body_truncated"`
	Hits          []DiagnosticHit `json:"hits"`
}

type DiagnosticHit struct {
	Category string `json:"category"`
	RuleID   string `json:"rule_id"`
	Severity string `json:"severity"`
	Observe  bool   `json:"observe"`
	Trusted  bool   `json:"trusted"`
	Source   string `json:"source,omitempty"`
	Code     string `json:"code,omitempty"`
}
