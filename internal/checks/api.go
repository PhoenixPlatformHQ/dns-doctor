// Package checks implements all DNS Doctor diagnostic checks.
package checks

import (
	"context"
	"fmt"

	"github.com/phoenix-platform/dns-doctor/internal/output"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd/api"
)

// RunAPIChecks runs check 1 (API reachable) and check 2 (context + cluster name).
// It returns the results together with the resolved cluster and context names.
func RunAPIChecks(ctx context.Context, client kubernetes.Interface, rawCfg *api.Config) (results []output.CheckResult, cluster, contextName string) {
	// ── Check 1: Kubernetes API reachable ──────────────────────────────────────
	sv, err := client.Discovery().ServerVersion()
	if err != nil {
		results = append(results, output.CheckResult{
			Name:        "Kubernetes API reachable",
			Status:      output.StatusFail,
			Fact:        "API server did not respond to the version check",
			Evidence:    err.Error(),
			FaultDomain: "Network connectivity, kubeconfig credentials, or cluster availability",
			NextCheck:   "Verify kubeconfig with: kubectl cluster-info",
		})
		// Without API access no further checks can succeed; return early.
		return results, "", ""
	}

	results = append(results, output.CheckResult{
		Name:     "Kubernetes API reachable",
		Status:   output.StatusPass,
		Fact:     "API server responded to version check",
		Evidence: fmt.Sprintf("Server version %s", sv.GitVersion),
	})

	// ── Check 2: Current context and cluster name ──────────────────────────────
	if rawCfg == nil {
		results = append(results, output.CheckResult{
			Name:     "Kubernetes context and cluster",
			Status:   output.StatusInfo,
			Fact:     "kubeconfig raw config not available",
			Evidence: "context and cluster name could not be determined",
		})
		return results, "", ""
	}

	contextName = rawCfg.CurrentContext
	if contextName == "" {
		results = append(results, output.CheckResult{
			Name:      "Kubernetes context and cluster",
			Status:    output.StatusWarn,
			Fact:      "No current-context is set in kubeconfig",
			Evidence:  "CurrentContext field is empty",
			NextCheck: "Set a context with: kubectl config use-context <name>",
		})
		return results, "", ""
	}

	ctxObj, ok := rawCfg.Contexts[contextName]
	if ok && ctxObj != nil {
		cluster = ctxObj.Cluster
	}

	results = append(results, output.CheckResult{
		Name:     "Kubernetes context and cluster",
		Status:   output.StatusInfo,
		Fact:     "Active kubeconfig context and cluster resolved",
		Evidence: fmt.Sprintf("context=%s  cluster=%s", contextName, cluster),
	})

	return results, cluster, contextName
}
