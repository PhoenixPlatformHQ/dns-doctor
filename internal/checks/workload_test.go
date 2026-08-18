package checks_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/phoenix-platform/dns-doctor/internal/checks"
	"github.com/phoenix-platform/dns-doctor/internal/output"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func makeService(ns, name, clusterIP string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec:       corev1.ServiceSpec{ClusterIP: clusterIP},
	}
}

func makeEndpointsWithAddrs(ns, name string, count int) *corev1.Endpoints {
	ep := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
	}
	if count > 0 {
		addrs := make([]corev1.EndpointAddress, count)
		for i := range addrs {
			addrs[i] = corev1.EndpointAddress{IP: fmt.Sprintf("10.0.0.%d", i+1)}
		}
		ep.Subsets = []corev1.EndpointSubset{{Addresses: addrs}}
	}
	return ep
}

func makePodWithDNSConfig(ns, name string, cfg *corev1.PodDNSConfig) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec:       corev1.PodSpec{DNSPolicy: corev1.DNSClusterFirst},
	}
	if cfg != nil {
		pod.Spec.DNSConfig = cfg
	}
	return pod
}

// ── Check 11: service/endpoint consistency ────────────────────────────────────

func TestRunWorkloadChecks_ServiceWithEndpoints(t *testing.T) {
	svc := makeService("default", "my-svc", "10.96.1.1")
	ep := makeEndpointsWithAddrs("default", "my-svc", 2)
	client := fake.NewSimpleClientset(svc, ep)

	results := checks.RunWorkloadChecks(context.Background(), client, "default", "")
	for _, r := range results {
		if strings.Contains(r.Name, "my-svc") && r.Status != output.StatusPass {
			t.Errorf("expected PASS for service with endpoints, got %s: %s", r.Status, r.Fact)
		}
	}
}

func TestRunWorkloadChecks_ServiceWithNoEndpoints(t *testing.T) {
	svc := makeService("default", "empty-svc", "10.96.2.2")
	ep := makeEndpointsWithAddrs("default", "empty-svc", 0)
	client := fake.NewSimpleClientset(svc, ep)

	results := checks.RunWorkloadChecks(context.Background(), client, "default", "")
	found := false
	for _, r := range results {
		if strings.Contains(r.Name, "empty-svc") && r.Status == output.StatusWarn {
			found = true
		}
	}
	if !found {
		t.Error("expected WARN for service with no endpoints")
	}
}

// ── Check 12: pod DNS config ──────────────────────────────────────────────────

func TestRunWorkloadChecks_NoPodFlag(t *testing.T) {
	client := fake.NewSimpleClientset()
	results := checks.RunWorkloadChecks(context.Background(), client, "default", "")

	for _, r := range results {
		if r.Name == "Pod DNS configuration" && r.Status != output.StatusInfo {
			t.Errorf("expected INFO when no pod specified, got %s", r.Status)
		}
	}
}

func TestRunWorkloadChecks_PodDefaultDNS(t *testing.T) {
	pod := makePodWithDNSConfig("default", "my-pod", nil)
	client := fake.NewSimpleClientset(pod)

	results := checks.RunWorkloadChecks(context.Background(), client, "default", "my-pod")
	found := false
	for _, r := range results {
		if r.Name == "Pod DNS configuration" && r.Status == output.StatusPass {
			found = true
		}
	}
	if !found {
		t.Error("expected PASS for pod with default DNS config")
	}
}

func TestRunWorkloadChecks_PodCustomNameservers(t *testing.T) {
	cfg := &corev1.PodDNSConfig{
		Nameservers: []string{"1.1.1.1"},
	}
	pod := makePodWithDNSConfig("default", "custom-dns-pod", cfg)
	client := fake.NewSimpleClientset(pod)

	results := checks.RunWorkloadChecks(context.Background(), client, "default", "custom-dns-pod")
	found := false
	for _, r := range results {
		if strings.Contains(r.Name, "custom nameservers") && r.Status == output.StatusWarn {
			found = true
		}
	}
	if !found {
		t.Error("expected WARN for pod with custom nameservers")
	}
}

func TestRunWorkloadChecks_PodNotFound(t *testing.T) {
	client := fake.NewSimpleClientset()
	results := checks.RunWorkloadChecks(context.Background(), client, "default", "missing-pod")
	found := false
	for _, r := range results {
		if r.Name == "Pod DNS configuration" && r.Status == output.StatusFail {
			found = true
		}
	}
	if !found {
		t.Error("expected FAIL for missing pod")
	}
}
