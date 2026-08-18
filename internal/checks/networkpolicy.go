package checks

import (
	"context"
	"fmt"

	"github.com/phoenix-platform/dns-doctor/internal/output"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// RunNetworkPolicyChecks implements check 10: NetworkPolicy presence in the
// target namespace. The tool lists — but does not evaluate — policy rules,
// because determining whether a policy blocks DNS requires rule analysis that
// depends on the CNI implementation and is not deterministic from the API alone.
func RunNetworkPolicyChecks(ctx context.Context, client kubernetes.Interface, namespace string) []output.CheckResult {
	ns := namespace
	if ns == "" {
		ns = "default"
	}

	policies, err := client.NetworkingV1().NetworkPolicies(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		if errors.IsForbidden(err) {
			return []output.CheckResult{{
				Name:     "NetworkPolicy presence",
				Status:   output.StatusWarn,
				Fact:     fmt.Sprintf("Insufficient permissions to read NetworkPolicies in namespace %q — skipping check", ns),
				Evidence: err.Error(),
			}}
		}
		return []output.CheckResult{{
			Name:     "NetworkPolicy presence",
			Status:   output.StatusFail,
			Fact:     fmt.Sprintf("Failed to list NetworkPolicies in namespace %q", ns),
			Evidence: err.Error(),
		}}
	}

	if len(policies.Items) == 0 {
		return []output.CheckResult{{
			Name:     "NetworkPolicy presence",
			Status:   output.StatusPass,
			Fact:     fmt.Sprintf("No NetworkPolicies found in namespace %q", ns),
			Evidence: "NetworkPolicy list returned 0 items — DNS egress is not restricted by a NetworkPolicy in this namespace",
		}}
	}

	names := make([]string, 0, len(policies.Items))
	for _, p := range policies.Items {
		names = append(names, p.Name)
	}

	return []output.CheckResult{{
		Name:        "NetworkPolicy presence",
		Status:      output.StatusWarn,
		Fact:        fmt.Sprintf("NetworkPolicies are present in namespace %q and may affect DNS egress — validate UDP/TCP port 53 access", ns),
		Evidence:    fmt.Sprintf("%d NetworkPolicy object(s) found: %v", len(policies.Items), names),
		FaultDomain: "A NetworkPolicy may be restricting egress to port 53 (UDP/TCP) required for DNS resolution",
		NextCheck:   fmt.Sprintf("kubectl get networkpolicy -n %s -o yaml   # review egress rules for port 53", ns),
	}}
}
