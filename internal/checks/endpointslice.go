package checks

import (
	"context"
	"fmt"

	"github.com/phoenix-platform/dns-doctor/internal/output"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// endpointSliceResult is the outcome of aggregating all EndpointSlices for one
// Service. It is consumed by both CoreDNS and workload checks.
type endpointSliceResult struct {
	// ready is the count of endpoints whose condition.ready is true or nil
	// (nil means "ready" per Kubernetes semantics — the field was absent in
	// older cluster versions).
	ready int
	// total is the count of all endpoint addresses seen across all slices.
	total int
	// sliceCount is the number of EndpointSlice objects matched.
	sliceCount int
	// forbidden is set when the API returned a Forbidden error.
	forbidden bool
	// forbiddenErr carries the original error message when forbidden is true.
	forbiddenErr string
}

// countReadyEndpointSlices lists all EndpointSlices in namespace that belong to
// serviceName (identified by the label kubernetes.io/service-name) and
// aggregates ready endpoint counts across all matching slices.
//
// Kubernetes EndpointSlice semantics (KEP-752):
//   - endpoint.Conditions.Ready == nil  →  treat as ready (backwards compat)
//   - endpoint.Conditions.Ready == true →  ready
//   - endpoint.Conditions.Ready == false → not ready
//
// Both IPv4 and IPv6 slices are counted; address type is not filtered.
func countReadyEndpointSlices(ctx context.Context, client kubernetes.Interface, namespace, serviceName string) endpointSliceResult {
	labelSel := fmt.Sprintf("kubernetes.io/service-name=%s", serviceName)
	slices, err := client.DiscoveryV1().EndpointSlices(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSel,
	})
	if err != nil {
		if errors.IsForbidden(err) {
			return endpointSliceResult{forbidden: true, forbiddenErr: err.Error()}
		}
		// Any non-forbidden error: return zero counts (caller decides fallback).
		return endpointSliceResult{}
	}

	var res endpointSliceResult
	res.sliceCount = len(slices.Items)
	for _, sl := range slices.Items {
		for _, ep := range sl.Endpoints {
			res.total++
			// nil Ready == ready per KEP-752 backwards compatibility guarantee.
			if ep.Conditions.Ready == nil || *ep.Conditions.Ready {
				res.ready++
			}
		}
	}
	return res
}

// endpointSliceCheckResult produces a []output.CheckResult for a single service
// using the EndpointSlice API as primary source.
//
// If EndpointSlices are forbidden, the check is marked [WARN] (unavailable).
// If no slices exist at all, the fallback caller should proceed to legacy Endpoints.
//
// Returns (results, shouldFallback):
//   - shouldFallback=true means no slices were found and legacy Endpoints should
//     be tried to distinguish "no slices yet" from "no backing pods".
func endpointSliceCheckResult(
	esr endpointSliceResult,
	checkName, namespace, serviceName string,
) (results []output.CheckResult, shouldFallback bool) {
	if esr.forbidden {
		return []output.CheckResult{{
			Name:     checkName,
			Status:   output.StatusWarn,
			Fact:     fmt.Sprintf("Insufficient permissions to read EndpointSlices in namespace %q — check coverage unavailable", namespace),
			Evidence: esr.forbiddenErr,
			NextCheck: fmt.Sprintf(
				"Grant list on discovery.k8s.io/endpointslices in %s, then re-run dns-doctor", namespace),
		}}, false
	}

	if esr.sliceCount == 0 {
		// No slices found — might mean the EndpointSlice controller hasn't run
		// yet or the cluster is old. Signal caller to try legacy fallback.
		return nil, true
	}

	if esr.ready == 0 {
		return []output.CheckResult{{
			Name:        checkName,
			Status:      output.StatusWarn,
			Fact:        fmt.Sprintf("Service %q has no ready endpoints across %d EndpointSlice(s) — connections to this service may fail", serviceName, esr.sliceCount),
			Evidence:    fmt.Sprintf("totalEndpoints=%d  readyEndpoints=0  slices=%d", esr.total, esr.sliceCount),
			FaultDomain: "No pods are Ready and selected by this Service",
			NextCheck:   fmt.Sprintf("kubectl get endpointslices -n %s -l kubernetes.io/service-name=%s", namespace, serviceName),
		}}, false
	}

	return []output.CheckResult{{
		Name:     checkName,
		Status:   output.StatusPass,
		Fact:     fmt.Sprintf("Service %q has %d ready endpoint(s) across %d EndpointSlice(s)", serviceName, esr.ready, esr.sliceCount),
		Evidence: fmt.Sprintf("readyEndpoints=%d  totalEndpoints=%d  slices=%d", esr.ready, esr.total, esr.sliceCount),
	}}, false
}
