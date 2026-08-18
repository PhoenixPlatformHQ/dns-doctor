package checks

import (
	"fmt"
	"strings"

	"github.com/phoenix-platform/dns-doctor/internal/output"
)

// RunSummaryCheck implements check 13: build an actionable next-step summary
// based on the aggregated results of all preceding checks.
func RunSummaryCheck(results []output.CheckResult) output.CheckResult {
	var fails, warns int
	var nextSteps []string

	for _, r := range results {
		switch r.Status {
		case output.StatusFail:
			fails++
			if r.NextCheck != "" {
				nextSteps = append(nextSteps, r.NextCheck)
			}
		case output.StatusWarn:
			warns++
		}
	}

	if fails == 0 && warns == 0 {
		return output.CheckResult{
			Name:     "Diagnostic summary",
			Status:   output.StatusPass,
			Fact:     "All checks passed — no DNS anomalies detected",
			Evidence: "0 FAIL  0 WARN",
			NextCheck: "If you are still experiencing DNS issues, consider running with --probe " +
				"(once implemented) to perform active DNS resolution tests from within the cluster",
		}
	}

	var sb strings.Builder
	if fails > 0 {
		sb.WriteString(fmt.Sprintf("%d FAIL  ", fails))
	}
	if warns > 0 {
		sb.WriteString(fmt.Sprintf("%d WARN  ", warns))
	}

	var nextCheckSummary string
	if len(nextSteps) > 0 {
		// Deduplicate.
		seen := make(map[string]struct{})
		var deduped []string
		for _, s := range nextSteps {
			if _, ok := seen[s]; !ok {
				seen[s] = struct{}{}
				deduped = append(deduped, s)
			}
		}
		// Return only the first three to avoid overwhelming output.
		if len(deduped) > 3 {
			deduped = deduped[:3]
		}
		nextCheckSummary = strings.Join(deduped, "  |  ")
	} else {
		nextCheckSummary = "Review the FAIL/WARN items above and follow each 'Next check' suggestion"
	}

	status := output.StatusWarn
	if fails > 0 {
		status = output.StatusFail
	}

	return output.CheckResult{
		Name:        "Diagnostic summary",
		Status:      status,
		Fact:        fmt.Sprintf("DNS Doctor detected potential issues — %s", strings.TrimSpace(sb.String())),
		Evidence:    fmt.Sprintf("total checks evaluated: %d", len(results)),
		FaultDomain: "Review individual check results above for specific fault domains",
		NextCheck:   nextCheckSummary,
	}
}
