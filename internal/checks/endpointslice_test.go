package checks_test

import (
	"context"
	"strings"
	"testing"

	"github.com/phoenix-platform/dns-doctor/internal/checks"
	"github.com/phoenix-platform/dns-doctor/internal/output"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// boolPtr is a helper to create a *bool from a literal.
func boolPtr(b bool) *bool { return &b }

// validCorefile is a minimal valid Corefile used in CoreDNS check tests.
const validCorefile = ".:53 {\n  forward . 8.8.8.8\n  loop\n}"

// makeSlice builds a minimal EndpointSlice tied to serviceName in the given
// namespace. endpoints is a list of (ip, ready) pairs.
func makeSlice(ns, serviceName, sliceName string, endpoints []sliceEndpoint) *discoveryv1.EndpointSlice {
	sl := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sliceName,
			Namespace: ns,
			Labels:    map[string]string{"kubernetes.io/service-name": serviceName},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
	}
	for _, e := range endpoints {
		ep := discoveryv1.Endpoint{
			Addresses:  []string{e.ip},
			Conditions: discoveryv1.EndpointConditions{Ready: e.ready},
		}
		sl.Endpoints = append(sl.Endpoints, ep)
	}
	return sl
}

type sliceEndpoint struct {
	ip    string
	ready *bool // nil means "ready" per KEP-752
}

// ── Test A: one healthy EndpointSlice ─────────────────────────────────────────

func TestEndpointSlice_A_OneHealthySlice(t *testing.T) {
	sl := makeSlice("default", "my-svc", "my-svc-abc", []sliceEndpoint{
		{ip: "10.0.0.1", ready: boolPtr(true)},
		{ip: "10.0.0.2", ready: boolPtr(true)},
	})
	svc := makeService("default", "my-svc", "10.96.1.1")
	client := fake.NewSimpleClientset(sl, svc)

	results := checks.RunWorkloadChecks(context.Background(), client, "default", "")
	assertEPSStatus(t, results, output.StatusPass, "TestA: expected PASS for service with 2 ready endpoints")
}

// ── Test B: multiple EndpointSlices ───────────────────────────────────────────

func TestEndpointSlice_B_MultipleSlices(t *testing.T) {
	sl1 := makeSlice("default", "my-svc", "my-svc-s1", []sliceEndpoint{
		{ip: "10.0.0.1", ready: boolPtr(true)},
	})
	sl2 := makeSlice("default", "my-svc", "my-svc-s2", []sliceEndpoint{
		{ip: "10.0.0.2", ready: boolPtr(true)},
		{ip: "10.0.0.3", ready: boolPtr(true)},
	})
	svc := makeService("default", "my-svc", "10.96.1.1")
	client := fake.NewSimpleClientset(sl1, sl2, svc)

	results := checks.RunWorkloadChecks(context.Background(), client, "default", "")
	assertEPSStatus(t, results, output.StatusPass, "TestB: expected PASS for service with endpoints across multiple slices")
	assertEPSFactContains(t, results, "3", "TestB: expected total of 3 ready endpoints mentioned")
}

// ── Test C: ready + non-ready endpoints ───────────────────────────────────────

func TestEndpointSlice_C_ReadyAndNotReady(t *testing.T) {
	sl := makeSlice("default", "my-svc", "my-svc-mix", []sliceEndpoint{
		{ip: "10.0.0.1", ready: boolPtr(true)},
		{ip: "10.0.0.2", ready: boolPtr(false)}, // not ready — must NOT be counted
		{ip: "10.0.0.3", ready: boolPtr(true)},
	})
	svc := makeService("default", "my-svc", "10.96.1.1")
	client := fake.NewSimpleClientset(sl, svc)

	results := checks.RunWorkloadChecks(context.Background(), client, "default", "")
	assertEPSStatus(t, results, output.StatusPass, "TestC: expected PASS — 2 of 3 endpoints are ready")
	assertEPSFactContains(t, results, "2", "TestC: expected 2 ready endpoints counted")
}

// ── Test D: no EndpointSlices at all → fallback to legacy Endpoints ───────────

func TestEndpointSlice_D_NoSlices_LegacyFallback(t *testing.T) {
	// No EndpointSlice objects; provide legacy Endpoints with 1 address.
	svc := makeService("default", "my-svc", "10.96.1.1")
	ep := makeEndpointsWithAddrs("default", "my-svc", 1)
	client := fake.NewSimpleClientset(svc, ep)

	results := checks.RunWorkloadChecks(context.Background(), client, "default", "")
	assertEPSStatus(t, results, output.StatusPass, "TestD: expected PASS via legacy Endpoints fallback")
	assertEPSFactContains(t, results, "fallback", "TestD: expected 'fallback' in result fact")
}

// ── Test E: empty EndpointSlice (zero endpoints in slice) ─────────────────────

func TestEndpointSlice_E_EmptySlice(t *testing.T) {
	// A slice exists but has no endpoints at all.
	sl := makeSlice("default", "my-svc", "my-svc-empty", []sliceEndpoint{})
	svc := makeService("default", "my-svc", "10.96.1.1")
	client := fake.NewSimpleClientset(sl, svc)

	results := checks.RunWorkloadChecks(context.Background(), client, "default", "")
	// One slice exists but 0 ready endpoints → WARN, not PASS.
	assertEPSStatus(t, results, output.StatusWarn, "TestE: expected WARN for empty EndpointSlice")
}

// ── Test F: EndpointSlice not accessible + no legacy Endpoints ────────────────
//
// Full Forbidden simulation requires an httptest mock server (known limitation).
// This test verifies graceful degradation when no endpoint data is available:
// the tool must not FAIL and must not crash.

func TestEndpointSlice_F_NoEndpointData(t *testing.T) {
	svc := makeService("default", "my-svc", "10.96.1.1")
	// No EndpointSlice and no Endpoints — fake returns NotFound for legacy Endpoints.
	client := fake.NewSimpleClientset(svc)

	results := checks.RunWorkloadChecks(context.Background(), client, "default", "")
	// Must not crash; WARN is acceptable (data unavailable), FAIL is not.
	assertNoEPSStatus(t, results, output.StatusFail, "TestF: tool must not FAIL when endpoint data is unavailable")
}

// ── Test G: IPv4 addresses ────────────────────────────────────────────────────

func TestEndpointSlice_G_IPv4(t *testing.T) {
	sl := makeSlice("default", "my-svc", "my-svc-v4", []sliceEndpoint{
		{ip: "192.168.1.10", ready: boolPtr(true)},
	})
	sl.AddressType = discoveryv1.AddressTypeIPv4
	svc := makeService("default", "my-svc", "10.96.1.1")
	client := fake.NewSimpleClientset(sl, svc)

	results := checks.RunWorkloadChecks(context.Background(), client, "default", "")
	assertEPSStatus(t, results, output.StatusPass, "TestG: expected PASS for IPv4 EndpointSlice")
}

// ── Test H: IPv6 addresses ────────────────────────────────────────────────────

func TestEndpointSlice_H_IPv6(t *testing.T) {
	sl := makeSlice("default", "my-svc", "my-svc-v6", []sliceEndpoint{
		{ip: "fd00::1", ready: boolPtr(true)},
	})
	sl.AddressType = discoveryv1.AddressTypeIPv6
	svc := makeService("default", "my-svc", "10.96.1.1")
	client := fake.NewSimpleClientset(sl, svc)

	results := checks.RunWorkloadChecks(context.Background(), client, "default", "")
	assertEPSStatus(t, results, output.StatusPass, "TestH: expected PASS for IPv6 EndpointSlice")
}

// ── Test I: mixed slices — one empty, one with ready endpoints ────────────────

func TestEndpointSlice_I_MixedSlices(t *testing.T) {
	// One completely empty slice + one slice with a ready endpoint.
	// Must NOT report zero ready just because one slice is empty.
	slEmpty := makeSlice("default", "my-svc", "my-svc-empty", []sliceEndpoint{})
	slReady := makeSlice("default", "my-svc", "my-svc-ready", []sliceEndpoint{
		{ip: "10.0.0.5", ready: boolPtr(true)},
	})
	svc := makeService("default", "my-svc", "10.96.1.1")
	client := fake.NewSimpleClientset(slEmpty, slReady, svc)

	results := checks.RunWorkloadChecks(context.Background(), client, "default", "")
	assertEPSStatus(t, results, output.StatusPass, "TestI: must aggregate across all slices, not stop at empty one")
	assertEPSFactContains(t, results, "1", "TestI: expected 1 ready endpoint across 2 slices")
}

// ── Test: nil Ready field → treated as ready (KEP-752) ────────────────────────

func TestEndpointSlice_NilReady_TreatedAsReady(t *testing.T) {
	sl := makeSlice("default", "my-svc", "my-svc-nil", []sliceEndpoint{
		{ip: "10.0.0.1", ready: nil}, // nil = ready per KEP-752
	})
	svc := makeService("default", "my-svc", "10.96.1.1")
	client := fake.NewSimpleClientset(sl, svc)

	results := checks.RunWorkloadChecks(context.Background(), client, "default", "")
	assertEPSStatus(t, results, output.StatusPass, "TestNilReady: nil Ready must be treated as ready per KEP-752")
}

// ── CoreDNS check 7 with EndpointSlice ────────────────────────────────────────

func TestCoreDNSEndpoints_EndpointSlice_Primary(t *testing.T) {
	svc := makeDNSService()
	sl := makeSlice("kube-system", "kube-dns", "kube-dns-abc", []sliceEndpoint{
		{ip: "10.244.0.1", ready: boolPtr(true)},
		{ip: "10.244.0.2", ready: boolPtr(true)},
	})
	pod := makeCoreDNSPod("coredns-abc", true, 0)
	cm := makeCoreDNSConfigMap(validCorefile)
	client := fake.NewSimpleClientset(svc, sl, pod, cm)

	results := checks.RunCoreDNSChecks(context.Background(), client, "")
	assertCheckNameStatus(t, results, "CoreDNS endpoints availability", output.StatusPass,
		"CoreDNS check 7 should PASS via EndpointSlice")
}

func TestCoreDNSEndpoints_EndpointSlice_NoReady(t *testing.T) {
	svc := makeDNSService()
	// Slice exists but all endpoints not ready.
	sl := makeSlice("kube-system", "kube-dns", "kube-dns-bad", []sliceEndpoint{
		{ip: "10.244.0.1", ready: boolPtr(false)},
	})
	pod := makeCoreDNSPod("coredns-abc", true, 0)
	cm := makeCoreDNSConfigMap(validCorefile)
	client := fake.NewSimpleClientset(svc, sl, pod, cm)

	results := checks.RunCoreDNSChecks(context.Background(), client, "")
	assertCheckNameStatus(t, results, "CoreDNS endpoints availability", output.StatusWarn,
		"CoreDNS check 7 should WARN when all endpoints not ready")
}

// ── helpers ───────────────────────────────────────────────────────────────────

func assertEPSStatus(t *testing.T, results []output.CheckResult, want output.Status, msg string) {
	t.Helper()
	for _, r := range results {
		if r.Status == want {
			return
		}
	}
	t.Errorf("%s\ngot statuses: %v", msg, epsStatusList(results))
}

func assertNoEPSStatus(t *testing.T, results []output.CheckResult, bad output.Status, msg string) {
	t.Helper()
	for _, r := range results {
		if r.Status == bad {
			t.Errorf("%s — unexpected %s in result: %+v", msg, bad, r)
		}
	}
}

func assertEPSFactContains(t *testing.T, results []output.CheckResult, substr, msg string) {
	t.Helper()
	for _, r := range results {
		if strings.Contains(r.Fact, substr) || strings.Contains(r.Evidence, substr) {
			return
		}
	}
	t.Errorf("%s\nsubstring %q not found in any Fact/Evidence field\nresults: %+v", msg, substr, results)
}

func assertCheckNameStatus(t *testing.T, results []output.CheckResult, checkName string, want output.Status, msg string) {
	t.Helper()
	for _, r := range results {
		if r.Name == checkName {
			if r.Status != want {
				t.Errorf("%s\ncheck %q: got %s, want %s\nresult: %+v", msg, checkName, r.Status, want, r)
			}
			return
		}
	}
	t.Errorf("%s\ncheck %q not found in results", msg, checkName)
}

func epsStatusList(results []output.CheckResult) []string {
	out := make([]string, len(results))
	for i, r := range results {
		out[i] = string(r.Status) + " " + r.Name
	}
	return out
}
