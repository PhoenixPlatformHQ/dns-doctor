package checks

import (
	"context"
	"fmt"

	"github.com/phoenix-platform/dns-doctor/internal/output"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// RunWorkloadChecks runs checks 11 (service/endpoint consistency) and 12 (pod
// DNS config).  When pod is empty, check 11 lists all services in the namespace
// and flags any that have zero endpoints.  Check 12 reads the pod spec's
// dnsConfig field — no exec is required.
func RunWorkloadChecks(ctx context.Context, client kubernetes.Interface, namespace, pod string) []output.CheckResult {
	ns := namespace
	if ns == "" {
		ns = "default"
	}
	var results []output.CheckResult
	results = append(results, checkServiceEndpoints(ctx, client, ns)...)
	results = append(results, checkPodDNSConfig(ctx, client, ns, pod)...)
	return results
}

// checkServiceEndpoints implements check 11: service/endpoint consistency.
//
// Strategy per service:
//  1. Try EndpointSlice (discovery.k8s.io/v1) — primary.
//  2. If no EndpointSlices found for a service, fall back to legacy core/v1 Endpoints.
//  3. If EndpointSlices forbidden, mark unavailable — do not try legacy fallback
//     (would give a false "no endpoints" result using a different API path).
func checkServiceEndpoints(ctx context.Context, client kubernetes.Interface, namespace string) []output.CheckResult {
	svcs, err := client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		if errors.IsForbidden(err) {
			return []output.CheckResult{{
				Name:     "Service/endpoint consistency",
				Status:   output.StatusWarn,
				Fact:     fmt.Sprintf("Insufficient permissions to read Services in namespace %q — check coverage unavailable", namespace),
				Evidence: err.Error(),
			}}
		}
		return []output.CheckResult{{
			Name:     "Service/endpoint consistency",
			Status:   output.StatusFail,
			Fact:     fmt.Sprintf("Failed to list Services in namespace %q", namespace),
			Evidence: err.Error(),
		}}
	}

	if len(svcs.Items) == 0 {
		return []output.CheckResult{{
			Name:     "Service/endpoint consistency",
			Status:   output.StatusInfo,
			Fact:     fmt.Sprintf("No Services found in namespace %q", namespace),
			Evidence: "Service list returned 0 items",
		}}
	}

	var results []output.CheckResult
	for _, svc := range svcs.Items {
		// Skip headless and ExternalName services — they have no endpoint backing.
		if svc.Spec.ClusterIP == "None" || svc.Spec.Type == "ExternalName" {
			continue
		}
		checkName := fmt.Sprintf("Service %s/%s endpoint consistency", namespace, svc.Name)

		// ── Primary: EndpointSlice ─────────────────────────────────────────────
		esr := countReadyEndpointSlices(ctx, client, namespace, svc.Name)
		sliceResults, shouldFallback := endpointSliceCheckResult(esr, checkName, namespace, svc.Name)
		if !shouldFallback {
			results = append(results, sliceResults...)
			continue
		}

		// ── Fallback: legacy core/v1 Endpoints ────────────────────────────────
		ep, err := client.CoreV1().Endpoints(namespace).Get(ctx, svc.Name, metav1.GetOptions{})
		if err != nil {
			if errors.IsForbidden(err) {
				results = append(results, output.CheckResult{
					Name:     checkName,
					Status:   output.StatusWarn,
					Fact:     "Insufficient permissions to read Endpoints — check coverage unavailable",
					Evidence: err.Error(),
				})
				continue
			}
			results = append(results, output.CheckResult{
				Name:     checkName,
				Status:   output.StatusWarn,
				Fact:     "Could not retrieve Endpoints object",
				Evidence: err.Error(),
			})
			continue
		}

		readyCount := 0
		for _, subset := range ep.Subsets {
			readyCount += len(subset.Addresses)
		}

		if readyCount == 0 {
			results = append(results, output.CheckResult{
				Name:        checkName,
				Status:      output.StatusWarn,
				Fact:        fmt.Sprintf("Service %q has no ready endpoints (legacy Endpoints fallback) — connections to this service may fail", svc.Name),
				Evidence:    "EndpointSlice: 0 slices found; legacy Endpoints subsets contain zero ready addresses",
				FaultDomain: "No pods are Ready and selected by this Service",
				NextCheck:   fmt.Sprintf("kubectl get endpoints -n %s %s", namespace, svc.Name),
			})
		} else {
			results = append(results, output.CheckResult{
				Name:     checkName,
				Status:   output.StatusPass,
				Fact:     fmt.Sprintf("Service %q has %d ready endpoint(s) (legacy Endpoints fallback)", svc.Name, readyCount),
				Evidence: fmt.Sprintf("EndpointSlice: 0 slices found; legacyEndpoints readyAddresses=%d", readyCount),
			})
		}
	}

	if len(results) == 0 {
		return []output.CheckResult{{
			Name:     "Service/endpoint consistency",
			Status:   output.StatusInfo,
			Fact:     fmt.Sprintf("No ClusterIP services found in namespace %q to check", namespace),
			Evidence: "All services are headless or ExternalName",
		}}
	}
	return results
}

// checkPodDNSConfig implements check 12.
// It reads the pod spec's dnsConfig and dnsPolicy fields — no exec is performed.
func checkPodDNSConfig(ctx context.Context, client kubernetes.Interface, namespace, podName string) []output.CheckResult {
	if podName == "" {
		return []output.CheckResult{{
			Name:     "Pod DNS configuration",
			Status:   output.StatusInfo,
			Fact:     "No --pod flag supplied; pod DNS configuration check skipped",
			Evidence: "Pass --pod <name> to inspect a specific pod's DNS config",
		}}
	}

	pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		if errors.IsForbidden(err) {
			return []output.CheckResult{{
				Name:     "Pod DNS configuration",
				Status:   output.StatusWarn,
				Fact:     fmt.Sprintf("Insufficient permissions to read Pod %q in namespace %q — skipping check", podName, namespace),
				Evidence: err.Error(),
			}}
		}
		if errors.IsNotFound(err) {
			return []output.CheckResult{{
				Name:     "Pod DNS configuration",
				Status:   output.StatusFail,
				Fact:     fmt.Sprintf("Pod %q not found in namespace %q", podName, namespace),
				Evidence: err.Error(),
			}}
		}
		return []output.CheckResult{{
			Name:     "Pod DNS configuration",
			Status:   output.StatusFail,
			Fact:     fmt.Sprintf("Failed to get Pod %q in namespace %q", podName, namespace),
			Evidence: err.Error(),
		}}
	}

	var results []output.CheckResult

	dnsPolicy := string(pod.Spec.DNSPolicy)
	if dnsPolicy == "" {
		dnsPolicy = "ClusterFirst (default)"
	}

	cfg := pod.Spec.DNSConfig
	if cfg == nil || (len(cfg.Nameservers) == 0 && len(cfg.Searches) == 0 && len(cfg.Options) == 0) {
		results = append(results, output.CheckResult{
			Name:     "Pod DNS configuration",
			Status:   output.StatusPass,
			Fact:     fmt.Sprintf("Pod %q uses default DNS configuration", podName),
			Evidence: fmt.Sprintf("dnsPolicy=%s  dnsConfig=<none>", dnsPolicy),
		})
	} else {
		results = append(results, output.CheckResult{
			Name:     "Pod DNS configuration",
			Status:   output.StatusInfo,
			Fact:     fmt.Sprintf("Pod %q has a custom DNS configuration", podName),
			Evidence: fmt.Sprintf("dnsPolicy=%s  nameservers=%v  searches=%v  options=%v", dnsPolicy, cfg.Nameservers, cfg.Searches, dnsOptionNames(cfg.Options)),
		})

		// Flag custom nameservers — they may bypass CoreDNS entirely.
		if len(cfg.Nameservers) > 0 {
			results = append(results, output.CheckResult{
				Name:        "Pod DNS configuration — custom nameservers",
				Status:      output.StatusWarn,
				Fact:        fmt.Sprintf("Pod %q specifies custom nameservers which may bypass CoreDNS", podName),
				Evidence:    fmt.Sprintf("nameservers=%v", cfg.Nameservers),
				FaultDomain: "Custom nameservers override the cluster DNS service; in-cluster service names may not resolve",
				NextCheck:   fmt.Sprintf("kubectl get pod -n %s %s -o jsonpath='{.spec.dnsConfig}'", namespace, podName),
			})
		}
	}
	return results
}

// dnsOptionNames extracts the Name field from PodDNSConfigOption entries.
func dnsOptionNames(opts []corev1.PodDNSConfigOption) []string {
	names := make([]string, 0, len(opts))
	for _, o := range opts {
		if o.Name != "" {
			names = append(names, o.Name)
		}
	}
	return names
}
