package checks_test

import (
	"context"
	"strings"
	"testing"

	"github.com/phoenix-platform/dns-doctor/internal/checks"
	"github.com/phoenix-platform/dns-doctor/internal/output"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestRunNetworkPolicyChecks_NoPolicies(t *testing.T) {
	client := fake.NewSimpleClientset()
	results := checks.RunNetworkPolicyChecks(context.Background(), client, "default")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != output.StatusPass {
		t.Errorf("expected PASS when no NetworkPolicies present, got %s", results[0].Status)
	}
}

func TestRunNetworkPolicyChecks_PoliciesPresent(t *testing.T) {
	np := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deny-all",
			Namespace: "production",
		},
	}
	client := fake.NewSimpleClientset(np)
	results := checks.RunNetworkPolicyChecks(context.Background(), client, "production")

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != output.StatusWarn {
		t.Errorf("expected WARN when NetworkPolicies are present, got %s", results[0].Status)
	}
	// Must NOT claim the policy blocks DNS — only flag presence.
	if strings.Contains(results[0].Fact, "blocks") {
		t.Errorf("fact must not claim policy blocks DNS: %q", results[0].Fact)
	}
}

func TestRunNetworkPolicyChecks_DefaultNamespace(t *testing.T) {
	client := fake.NewSimpleClientset()
	// Empty namespace should default to "default".
	results := checks.RunNetworkPolicyChecks(context.Background(), client, "")
	if len(results) == 0 {
		t.Fatal("expected at least 1 result")
	}
}
