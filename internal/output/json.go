package output

import (
	"encoding/json"
	"io"
	"os"
	"time"
)

// JSONReport is the top-level JSON output document.
type JSONReport struct {
	Version   string        `json:"dns_doctor_version"`
	Timestamp string        `json:"timestamp"`
	Cluster   string        `json:"cluster"`
	Context   string        `json:"context"`
	Namespace string        `json:"namespace"`
	Results   []CheckResult `json:"results"`
	Summary   JSONSummary   `json:"summary"`
}

// JSONSummary holds per-status counts.
type JSONSummary struct {
	Pass int `json:"pass"`
	Warn int `json:"warn"`
	Fail int `json:"fail"`
	Info int `json:"info"`
}

// WriteJSON serialises the full report to the given writer (defaults to stdout when nil).
func WriteJSON(w io.Writer, version, cluster, context, namespace string, results []CheckResult) error {
	if w == nil {
		w = os.Stdout
	}
	pass, warn, fail, info := tally(results)
	report := JSONReport{
		Version:   version,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Cluster:   cluster,
		Context:   context,
		Namespace: namespace,
		Results:   results,
		Summary: JSONSummary{
			Pass: pass,
			Warn: warn,
			Fail: fail,
			Info: info,
		},
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}
