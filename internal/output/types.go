// Package output defines the shared result types used across all DNS Doctor checks.
package output

// Status represents the result classification of a single check.
type Status string

const (
	StatusPass Status = "PASS"
	StatusWarn Status = "WARN"
	StatusFail Status = "FAIL"
	StatusInfo Status = "INFO"
)

// CheckResult captures the full diagnostic output of a single check.
type CheckResult struct {
	Name        string `json:"name"`
	Status      Status `json:"status"`
	Fact        string `json:"fact"`
	Evidence    string `json:"evidence"`
	FaultDomain string `json:"likely_fault_domain,omitempty"`
	NextCheck   string `json:"next_check,omitempty"`
	Error       string `json:"error,omitempty"`
}
