package checks

import (
	"context"
	"fmt"
	"strings"

	"github.com/phoenix-platform/dns-doctor/internal/output"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// checkCoreDNSEndpoints implements check 7: CoreDNS endpoints availability.
// Strategy:
//  1. Try EndpointSlice (discovery.k8s.io/v1) — primary, avoids the deprecated
//     core/v1 Endpoints API warning on Kubernetes ≥ v1.33.
//  2. If no EndpointSlices are found (cluster pre-v1.21 or controller not yet
//     synced), fall back to the legacy core/v1 Endpoints API.
//  3. If EndpointSlices are forbidden, mark the check unavailable and stop;
//     do not attempt the legacy fallback to avoid a false reassurance.

const (
	dnsNamespace  = "kube-system"
	coreDNSCMName = "coredns"
)

// dnsServiceLabels lists the label selectors tried in order to find the DNS service.
var dnsServiceLabels = []map[string]string{
	{"k8s-app": "kube-dns"},
	{"app": "coredns"},
	{"app.kubernetes.io/name": "coredns"},
}

// RunCoreDNSChecks runs checks 3–9 covering the CoreDNS service, pods,
// endpoints, ConfigMap, and obvious configuration warnings.
func RunCoreDNSChecks(ctx context.Context, client kubernetes.Interface, _ string) []output.CheckResult {
	var results []output.CheckResult

	// ── Check 3: DNS service discovery ────────────────────────────────────────
	dnsSvc, svcResults := findDNSService(ctx, client)
	results = append(results, svcResults...)

	if dnsSvc == nil {
		// Without a DNS service the remaining checks cannot proceed meaningfully.
		results = append(results, output.CheckResult{
			Name:        "CoreDNS checks skipped",
			Status:      output.StatusInfo,
			Fact:        "Remaining CoreDNS checks skipped because DNS service was not found",
			Evidence:    "checks 4–9 require a resolvable DNS service",
			FaultDomain: "DNS service missing or inaccessible",
			NextCheck:   "Ensure kube-dns/CoreDNS is deployed in kube-system",
		})
		return results
	}

	// ── Check 4: DNS ClusterIP readable ───────────────────────────────────────
	clusterIP := dnsSvc.Spec.ClusterIP
	if clusterIP == "" || clusterIP == "None" {
		results = append(results, output.CheckResult{
			Name:        "DNS service ClusterIP",
			Status:      output.StatusFail,
			Fact:        "DNS service has no ClusterIP assigned",
			Evidence:    fmt.Sprintf("ClusterIP=%q", clusterIP),
			FaultDomain: "DNS service misconfiguration — headless service or pending allocation",
			NextCheck:   "kubectl get svc -n kube-system kube-dns -o yaml",
		})
	} else {
		results = append(results, output.CheckResult{
			Name:     "DNS service ClusterIP",
			Status:   output.StatusPass,
			Fact:     "DNS service has a valid ClusterIP",
			Evidence: fmt.Sprintf("ClusterIP=%s", clusterIP),
		})
	}

	// ── Checks 5 & 6: CoreDNS pod health and restart counts ───────────────────
	results = append(results, checkCoreDNSPods(ctx, client)...)

	// ── Check 7: CoreDNS endpoints availability ────────────────────────────────
	results = append(results, checkCoreDNSEndpoints(ctx, client, dnsSvc.Name)...)

	// ── Checks 8 & 9: CoreDNS ConfigMap + config warnings ─────────────────────
	results = append(results, checkCoreDNSConfigMap(ctx, client)...)

	return results
}

// findDNSService tries common label selectors to locate the in-cluster DNS service.
func findDNSService(ctx context.Context, client kubernetes.Interface) (*corev1.Service, []output.CheckResult) {
	var results []output.CheckResult

	// First, try to list services and find kube-dns or coredns by label.
	for _, labels := range dnsServiceLabels {
		sel := labelsToSelector(labels)
		svcs, err := client.CoreV1().Services(dnsNamespace).List(ctx, metav1.ListOptions{LabelSelector: sel})
		if err != nil {
			if errors.IsForbidden(err) {
				results = append(results, output.CheckResult{
					Name:     "Kubernetes DNS service discovery",
					Status:   output.StatusWarn,
					Fact:     "Insufficient permissions to read Services in kube-system — skipping check",
					Evidence: err.Error(),
				})
				return nil, results
			}
			continue
		}
		if len(svcs.Items) > 0 {
			svc := svcs.Items[0]
			results = append(results, output.CheckResult{
				Name:     "Kubernetes DNS service discovery",
				Status:   output.StatusPass,
				Fact:     "DNS service found in kube-system",
				Evidence: fmt.Sprintf("service/%s ClusterIP=%s", svc.Name, svc.Spec.ClusterIP),
			})
			return &svcs.Items[0], results
		}
	}

	// Fallback: try to GET kube-dns by name directly.
	svc, err := client.CoreV1().Services(dnsNamespace).Get(ctx, "kube-dns", metav1.GetOptions{})
	if err == nil {
		results = append(results, output.CheckResult{
			Name:     "Kubernetes DNS service discovery",
			Status:   output.StatusPass,
			Fact:     "DNS service found in kube-system (by name)",
			Evidence: fmt.Sprintf("service/kube-dns ClusterIP=%s", svc.Spec.ClusterIP),
		})
		return svc, results
	}

	if errors.IsForbidden(err) {
		results = append(results, output.CheckResult{
			Name:     "Kubernetes DNS service discovery",
			Status:   output.StatusWarn,
			Fact:     "Insufficient permissions to read Services in kube-system — skipping check",
			Evidence: err.Error(),
		})
		return nil, results
	}

	results = append(results, output.CheckResult{
		Name:        "Kubernetes DNS service discovery",
		Status:      output.StatusFail,
		Fact:        "No DNS service found in kube-system with expected labels or name",
		Evidence:    "Tried labels k8s-app=kube-dns, app=coredns, and GET kube-dns",
		FaultDomain: "CoreDNS or kube-dns may not be deployed",
		NextCheck:   "kubectl get svc -n kube-system",
	})
	return nil, results
}

// checkCoreDNSPods implements checks 5 and 6.
func checkCoreDNSPods(ctx context.Context, client kubernetes.Interface) []output.CheckResult {
	var results []output.CheckResult

	pods, err := client.CoreV1().Pods(dnsNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "k8s-app=kube-dns",
	})
	if err != nil {
		if errors.IsForbidden(err) {
			return append(results, output.CheckResult{
				Name:     "CoreDNS pod health",
				Status:   output.StatusWarn,
				Fact:     "Insufficient permissions to read Pods in kube-system — skipping check",
				Evidence: err.Error(),
			})
		}
		return append(results, output.CheckResult{
			Name:     "CoreDNS pod health",
			Status:   output.StatusFail,
			Fact:     "Failed to list CoreDNS pods in kube-system",
			Evidence: err.Error(),
		})
	}

	if len(pods.Items) == 0 {
		return append(results, output.CheckResult{
			Name:        "CoreDNS pod health",
			Status:      output.StatusFail,
			Fact:        "No CoreDNS pods found in kube-system with label k8s-app=kube-dns",
			Evidence:    "pod list returned 0 items",
			FaultDomain: "CoreDNS Deployment may be scaled to zero or incorrectly labelled",
			NextCheck:   "kubectl get pods -n kube-system -l k8s-app=kube-dns",
		})
	}

	for _, pod := range pods.Items {
		podName := pod.Name
		running := pod.Status.Phase == corev1.PodRunning

		// Check 5: pod Running/Ready.
		ready := podReady(&pod)
		if running && ready {
			results = append(results, output.CheckResult{
				Name:     "CoreDNS pod health",
				Status:   output.StatusPass,
				Fact:     fmt.Sprintf("CoreDNS pod %s is Running and Ready", podName),
				Evidence: fmt.Sprintf("phase=%s ready=true", pod.Status.Phase),
			})
		} else {
			results = append(results, output.CheckResult{
				Name:        "CoreDNS pod health",
				Status:      output.StatusFail,
				Fact:        fmt.Sprintf("CoreDNS pod %s is not Running/Ready", podName),
				Evidence:    fmt.Sprintf("phase=%s ready=%v", pod.Status.Phase, ready),
				FaultDomain: "CoreDNS pod may be crashing, pending, or evicted",
				NextCheck:   fmt.Sprintf("kubectl describe pod -n kube-system %s", podName),
			})
		}

		// Check 6: restart counts.
		for _, cs := range pod.Status.ContainerStatuses {
			restarts := cs.RestartCount
			if restarts == 0 {
				results = append(results, output.CheckResult{
					Name:     fmt.Sprintf("CoreDNS pod %s restart count", podName),
					Status:   output.StatusPass,
					Fact:     "CoreDNS pod restart count is zero",
					Evidence: fmt.Sprintf("restartCount=%d container=%s", restarts, cs.Name),
				})
			} else if restarts < 5 {
				results = append(results, output.CheckResult{
					Name:        fmt.Sprintf("CoreDNS pod %s restart count", podName),
					Status:      output.StatusWarn,
					Fact:        fmt.Sprintf("CoreDNS pod %s has %d restart(s)", podName, restarts),
					Evidence:    fmt.Sprintf("restartCount=%d container=%s", restarts, cs.Name),
					FaultDomain: "CoreDNS instability — may cause intermittent DNS failures",
					NextCheck:   fmt.Sprintf("kubectl logs -n kube-system %s --previous", podName),
				})
			} else {
				results = append(results, output.CheckResult{
					Name:        fmt.Sprintf("CoreDNS pod %s restart count", podName),
					Status:      output.StatusFail,
					Fact:        fmt.Sprintf("CoreDNS pod %s has elevated restart count (%d)", podName, restarts),
					Evidence:    fmt.Sprintf("restartCount=%d container=%s", restarts, cs.Name),
					FaultDomain: "Repeated CoreDNS crashes — DNS resolution is likely degraded",
					NextCheck:   fmt.Sprintf("kubectl logs -n kube-system %s --previous", podName),
				})
			}
		}
	}
	return results
}

func checkCoreDNSEndpoints(ctx context.Context, client kubernetes.Interface, svcName string) []output.CheckResult {
	const checkName = "CoreDNS endpoints availability"

	// ── Primary: EndpointSlice ─────────────────────────────────────────────────
	esr := countReadyEndpointSlices(ctx, client, dnsNamespace, svcName)
	results, shouldFallback := endpointSliceCheckResult(esr, checkName, dnsNamespace, svcName)
	if !shouldFallback {
		return results
	}

	// ── Fallback: legacy core/v1 Endpoints ────────────────────────────────────
	// Reached only when EndpointSlice returned zero slices (not forbidden).
	// This handles clusters that pre-date EndpointSlice or haven't yet synced.
	ep, err := client.CoreV1().Endpoints(dnsNamespace).Get(ctx, svcName, metav1.GetOptions{})
	if err != nil {
		if errors.IsForbidden(err) {
			return []output.CheckResult{{
				Name:     checkName,
				Status:   output.StatusWarn,
				Fact:     "Insufficient permissions to read Endpoints in kube-system — check coverage unavailable",
				Evidence: err.Error(),
				NextCheck: "Grant get on core/endpoints in kube-system, then re-run dns-doctor",
			}}
		}
		return []output.CheckResult{{
			Name:     checkName,
			Status:   output.StatusFail,
			Fact:     fmt.Sprintf("Failed to get Endpoints for service/%s in kube-system", svcName),
			Evidence: err.Error(),
		}}
	}

	var readyAddrs int
	for _, subset := range ep.Subsets {
		readyAddrs += len(subset.Addresses)
	}

	if readyAddrs == 0 {
		return []output.CheckResult{{
			Name:        checkName,
			Status:      output.StatusFail,
			Fact:        fmt.Sprintf("DNS service/%s has no ready endpoints (legacy Endpoints fallback)", svcName),
			Evidence:    "EndpointSlice: 0 slices found; legacy Endpoints subsets contain zero ready addresses",
			FaultDomain: "All CoreDNS pods may be unhealthy or not yet scheduled",
			NextCheck:   fmt.Sprintf("kubectl get endpoints -n kube-system %s", svcName),
		}}
	}

	return []output.CheckResult{{
		Name:     checkName,
		Status:   output.StatusPass,
		Fact:     fmt.Sprintf("DNS service/%s has %d ready endpoint(s) (legacy Endpoints fallback)", svcName, readyAddrs),
		Evidence: fmt.Sprintf("EndpointSlice: 0 slices found; legacyEndpoints readyAddresses=%d", readyAddrs),
	}}
}

// checkCoreDNSConfigMap implements checks 8 and 9.
func checkCoreDNSConfigMap(ctx context.Context, client kubernetes.Interface) []output.CheckResult {
	var results []output.CheckResult

	cm, err := client.CoreV1().ConfigMaps(dnsNamespace).Get(ctx, coreDNSCMName, metav1.GetOptions{})
	if err != nil {
		if errors.IsForbidden(err) {
			return append(results, output.CheckResult{
				Name:     "CoreDNS ConfigMap availability",
				Status:   output.StatusWarn,
				Fact:     "Insufficient permissions to read ConfigMap coredns in kube-system — skipping check",
				Evidence: err.Error(),
			})
		}
		if errors.IsNotFound(err) {
			return append(results, output.CheckResult{
				Name:        "CoreDNS ConfigMap availability",
				Status:      output.StatusFail,
				Fact:        "ConfigMap coredns not found in kube-system",
				Evidence:    "GET configmap/coredns returned 404",
				FaultDomain: "CoreDNS has no Corefile — will use built-in defaults or fail to start",
				NextCheck:   "kubectl get configmap -n kube-system coredns -o yaml",
			})
		}
		return append(results, output.CheckResult{
			Name:     "CoreDNS ConfigMap availability",
			Status:   output.StatusFail,
			Fact:     "Failed to read ConfigMap coredns in kube-system",
			Evidence: err.Error(),
		})
	}

	// Check 8: ConfigMap exists.
	results = append(results, output.CheckResult{
		Name:     "CoreDNS ConfigMap availability",
		Status:   output.StatusPass,
		Fact:     "ConfigMap coredns found in kube-system",
		Evidence: fmt.Sprintf("configmap/coredns  keys=%v", cmKeys(cm.Data)),
	})

	// Check 9: obvious Corefile config warnings.
	corefile, ok := cm.Data["Corefile"]
	if !ok || corefile == "" {
		results = append(results, output.CheckResult{
			Name:        "CoreDNS Corefile configuration",
			Status:      output.StatusWarn,
			Fact:        "Corefile key is absent or empty in ConfigMap coredns",
			Evidence:    "cm.Data[\"Corefile\"] is empty",
			FaultDomain: "CoreDNS will fall back to compiled-in defaults; custom zones or forwarders will be absent",
			NextCheck:   "kubectl edit configmap -n kube-system coredns",
		})
		return results
	}

	results = append(results, corefileWarnings(corefile)...)
	return results
}

// corefileWarnings checks for specific common CoreDNS configuration pitfalls.
func corefileWarnings(corefile string) []output.CheckResult {
	var results []output.CheckResult

	// Warn if no forward directive is present.
	if !strings.Contains(corefile, "forward") {
		results = append(results, output.CheckResult{
			Name:        "CoreDNS Corefile configuration",
			Status:      output.StatusWarn,
			Fact:        "No 'forward' directive found in Corefile",
			Evidence:    "keyword 'forward' absent from Corefile text",
			FaultDomain: "External DNS resolution may fail — CoreDNS cannot forward queries to upstream resolvers",
			NextCheck:   "Add a forward . <upstream-DNS> block to the Corefile",
		})
	}

	// Warn if the loop plugin is missing (loop detection prevents infinite loops).
	if !strings.Contains(corefile, "loop") {
		results = append(results, output.CheckResult{
			Name:        "CoreDNS Corefile configuration",
			Status:      output.StatusWarn,
			Fact:        "The 'loop' plugin is not present in Corefile",
			Evidence:    "keyword 'loop' absent from Corefile text",
			FaultDomain: "Without the loop plugin, CoreDNS cannot detect and halt forwarding loops",
			NextCheck:   "Add 'loop' to the default server block in the Corefile",
		})
	}

	if len(results) == 0 {
		results = append(results, output.CheckResult{
			Name:     "CoreDNS Corefile configuration",
			Status:   output.StatusPass,
			Fact:     "No obvious configuration warnings detected in Corefile",
			Evidence: "forward and loop directives are present",
		})
	}

	return results
}

// ── helpers ────────────────────────────────────────────────────────────────────

func labelsToSelector(labels map[string]string) string {
	var parts []string
	for k, v := range labels {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, ",")
}

func podReady(pod *corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

func cmKeys(data map[string]string) []string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	return keys
}
