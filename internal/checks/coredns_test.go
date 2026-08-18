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

func makeDNSService() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-dns",
			Namespace: "kube-system",
			Labels:    map[string]string{"k8s-app": "kube-dns"},
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "10.96.0.10",
		},
	}
}

func makeDNSEndpoints(svcName string, addresses int) *corev1.Endpoints {
	ep := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{Name: svcName, Namespace: "kube-system"},
	}
	if addresses > 0 {
		addrs := make([]corev1.EndpointAddress, addresses)
		for i := range addrs {
			addrs[i] = corev1.EndpointAddress{IP: fmt.Sprintf("10.244.0.%d", i+1)}
		}
		ep.Subsets = []corev1.EndpointSubset{{Addresses: addrs}}
	}
	return ep
}

func makeCoreDNSPod(name string, running bool, restarts int32) *corev1.Pod {
	phase := corev1.PodRunning
	if !running {
		phase = corev1.PodFailed
	}
	readyCond := corev1.ConditionTrue
	if !running {
		readyCond = corev1.ConditionFalse
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "kube-system",
			Labels:    map[string]string{"k8s-app": "kube-dns"},
		},
		Status: corev1.PodStatus{
			Phase: phase,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: readyCond},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "coredns", RestartCount: restarts},
			},
		},
	}
}

func makeCoreDNSConfigMap(corefile string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "coredns", Namespace: "kube-system"},
		Data:       map[string]string{"Corefile": corefile},
	}
}

// ── Check 3: DNS service discovery ───────────────────────────────────────────

func TestRunCoreDNSChecks_ServiceFound(t *testing.T) {
	svc := makeDNSService()
	ep := makeDNSEndpoints("kube-dns", 2)
	cm := makeCoreDNSConfigMap(".:53 {\n  forward . 8.8.8.8\n  loop\n}")
	pod := makeCoreDNSPod("coredns-abc", true, 0)
	client := fake.NewSimpleClientset(svc, ep, cm, pod)

	results := checks.RunCoreDNSChecks(context.Background(), client, "")
	assertAnyStatus(t, results, output.StatusPass)
}

func TestRunCoreDNSChecks_ServiceMissing(t *testing.T) {
	client := fake.NewSimpleClientset()
	results := checks.RunCoreDNSChecks(context.Background(), client, "")
	assertAnyStatus(t, results, output.StatusFail)
}

// ── Check 4: ClusterIP ───────────────────────────────────────────────────────

func TestRunCoreDNSChecks_NoClusterIP(t *testing.T) {
	svc := makeDNSService()
	svc.Spec.ClusterIP = ""
	ep := makeDNSEndpoints("kube-dns", 0)
	cm := makeCoreDNSConfigMap(".:53 { forward . 8.8.8.8\n loop\n}")
	pod := makeCoreDNSPod("coredns-abc", true, 0)
	client := fake.NewSimpleClientset(svc, ep, cm, pod)

	results := checks.RunCoreDNSChecks(context.Background(), client, "")
	assertStatusForName(t, results, "DNS service ClusterIP", output.StatusFail)
}

// ── Check 5 & 6: pod health + restarts ───────────────────────────────────────

func TestRunCoreDNSChecks_PodHighRestarts(t *testing.T) {
	svc := makeDNSService()
	ep := makeDNSEndpoints("kube-dns", 1)
	cm := makeCoreDNSConfigMap(".:53 { forward . 8.8.8.8\n loop\n}")
	pod := makeCoreDNSPod("coredns-abc", true, 10)
	client := fake.NewSimpleClientset(svc, ep, cm, pod)

	results := checks.RunCoreDNSChecks(context.Background(), client, "")
	assertStatusForNameContains(t, results, "restart", output.StatusFail)
}

func TestRunCoreDNSChecks_PodNotRunning(t *testing.T) {
	svc := makeDNSService()
	ep := makeDNSEndpoints("kube-dns", 0)
	cm := makeCoreDNSConfigMap(".:53 { forward . 8.8.8.8\n loop\n}")
	pod := makeCoreDNSPod("coredns-abc", false, 0)
	client := fake.NewSimpleClientset(svc, ep, cm, pod)

	results := checks.RunCoreDNSChecks(context.Background(), client, "")
	assertStatusForNameContains(t, results, "pod health", output.StatusFail)
}

// ── Check 7: endpoints ────────────────────────────────────────────────────────

func TestRunCoreDNSChecks_NoEndpoints(t *testing.T) {
	svc := makeDNSService()
	ep := makeDNSEndpoints("kube-dns", 0)
	cm := makeCoreDNSConfigMap(".:53 { forward . 8.8.8.8\n loop\n}")
	pod := makeCoreDNSPod("coredns-abc", true, 0)
	client := fake.NewSimpleClientset(svc, ep, cm, pod)

	results := checks.RunCoreDNSChecks(context.Background(), client, "")
	assertStatusForNameContains(t, results, "endpoints", output.StatusFail)
}

// ── Check 9: config warnings ──────────────────────────────────────────────────

func TestRunCoreDNSChecks_MissingForward(t *testing.T) {
	svc := makeDNSService()
	ep := makeDNSEndpoints("kube-dns", 1)
	cm := makeCoreDNSConfigMap(".:53 { loop\n}")
	pod := makeCoreDNSPod("coredns-abc", true, 0)
	client := fake.NewSimpleClientset(svc, ep, cm, pod)

	results := checks.RunCoreDNSChecks(context.Background(), client, "")
	assertStatusForNameContains(t, results, "Corefile", output.StatusWarn)
}

func TestRunCoreDNSChecks_MissingLoop(t *testing.T) {
	svc := makeDNSService()
	ep := makeDNSEndpoints("kube-dns", 1)
	cm := makeCoreDNSConfigMap(".:53 { forward . 8.8.8.8\n}")
	pod := makeCoreDNSPod("coredns-abc", true, 0)
	client := fake.NewSimpleClientset(svc, ep, cm, pod)

	results := checks.RunCoreDNSChecks(context.Background(), client, "")
	assertStatusForNameContains(t, results, "Corefile", output.StatusWarn)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func assertAnyStatus(t *testing.T, results []output.CheckResult, want output.Status) {
	t.Helper()
	for _, r := range results {
		if r.Status == want {
			return
		}
	}
	t.Errorf("expected at least one result with status %s; got %+v", want, coreStatusList(results))
}

func assertStatusForName(t *testing.T, results []output.CheckResult, name string, want output.Status) {
	t.Helper()
	for _, r := range results {
		if r.Name == name {
			if r.Status != want {
				t.Errorf("check %q: want %s, got %s", name, want, r.Status)
			}
			return
		}
	}
	t.Errorf("no result with name %q found", name)
}

func assertStatusForNameContains(t *testing.T, results []output.CheckResult, substr string, want output.Status) {
	t.Helper()
	for _, r := range results {
		if containsCI(r.Name, substr) && r.Status == want {
			return
		}
	}
	t.Errorf("expected a result containing %q with status %s; got %+v", substr, want, coreStatusList(results))
}

func coreStatusList(results []output.CheckResult) []string {
	out := make([]string, len(results))
	for i, r := range results {
		out[i] = fmt.Sprintf("[%s] %s", r.Status, r.Name)
	}
	return out
}

func containsCI(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		strings.Contains(strings.ToLower(s), strings.ToLower(substr)))
}
