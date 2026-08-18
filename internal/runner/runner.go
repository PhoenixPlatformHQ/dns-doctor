// Package runner orchestrates all DNS Doctor checks and collects results.
package runner

import (
	"context"

	"github.com/phoenix-platform/dns-doctor/internal/checks"
	"github.com/phoenix-platform/dns-doctor/internal/output"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd/api"
)

// Options holds the resolved CLI flags passed to every check.
type Options struct {
	Namespace string
	Pod       string
	ProbeMode bool
}

// Run executes all checks in order and returns the aggregated results together
// with the cluster and context names discovered during check 1.
func Run(ctx context.Context, client kubernetes.Interface, rawCfg *api.Config, opts Options) (results []output.CheckResult, cluster, contextName string) {
	// Checks 1 & 2: API reachable + context/cluster info.
	apiResults, clusterOut, ctxOut := checks.RunAPIChecks(ctx, client, rawCfg)
	results = append(results, apiResults...)
	cluster = clusterOut
	contextName = ctxOut

	// Checks 3–9: CoreDNS service, pods, endpoints, configmap, config warnings.
	results = append(results, checks.RunCoreDNSChecks(ctx, client, opts.Namespace)...)

	// Check 10: NetworkPolicy presence in the target namespace.
	results = append(results, checks.RunNetworkPolicyChecks(ctx, client, opts.Namespace)...)

	// Checks 11–12: Service/endpoint consistency and pod DNS config.
	results = append(results, checks.RunWorkloadChecks(ctx, client, opts.Namespace, opts.Pod)...)

	// Check 13: Actionable summary.
	results = append(results, checks.RunSummaryCheck(results))

	return results, cluster, contextName
}
