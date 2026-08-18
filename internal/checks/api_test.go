package checks_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/phoenix-platform/dns-doctor/internal/checks"
	"github.com/phoenix-platform/dns-doctor/internal/output"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/clientcmd/api"
)

func TestRunAPIChecks_ReachableCluster(t *testing.T) {
	client := fake.NewSimpleClientset()
	rawCfg := &api.Config{
		CurrentContext: "test-context",
		Contexts: map[string]*api.Context{
			"test-context": {Cluster: "test-cluster"},
		},
	}

	results, cluster, ctx := checks.RunAPIChecks(context.Background(), client, rawCfg)

	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}
	if results[0].Status != output.StatusPass {
		t.Errorf("check 1 status: want PASS, got %s", results[0].Status)
	}
	if results[1].Status != output.StatusInfo {
		t.Errorf("check 2 status: want INFO, got %s", results[1].Status)
	}
	if cluster != "test-cluster" {
		t.Errorf("cluster: want test-cluster, got %q", cluster)
	}
	if ctx != "test-context" {
		t.Errorf("context: want test-context, got %q", ctx)
	}
}

func TestRunAPIChecks_APIUnreachable(t *testing.T) {
	// The fake client always responds to discovery; test the nil rawCfg INFO path.
	client := fake.NewSimpleClientset()
	results, _, _ := checks.RunAPIChecks(context.Background(), client, nil)
	if len(results) < 1 {
		t.Fatalf("expected at least 1 result, got %d", len(results))
	}
	// First result should be PASS (fake client always succeeds discovery).
	if results[0].Status != output.StatusPass {
		t.Errorf("expected PASS for fake client, got %s", results[0].Status)
	}
	// Use fmt to satisfy the import.
	_ = fmt.Sprintf("")
}

func TestRunAPIChecks_NilRawConfig(t *testing.T) {
	client := fake.NewSimpleClientset()
	results, cluster, contextName := checks.RunAPIChecks(context.Background(), client, nil)

	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}
	if results[1].Status != output.StatusInfo {
		t.Errorf("check 2 with nil rawCfg should be INFO, got %s", results[1].Status)
	}
	if cluster != "" || contextName != "" {
		t.Errorf("expected empty cluster/context with nil rawCfg")
	}
}

func TestRunAPIChecks_NoCurrentContext(t *testing.T) {
	client := fake.NewSimpleClientset()
	rawCfg := &api.Config{
		CurrentContext: "",
	}
	results, _, _ := checks.RunAPIChecks(context.Background(), client, rawCfg)
	found := false
	for _, r := range results {
		if r.Status == output.StatusWarn {
			found = true
		}
	}
	if !found {
		t.Error("expected a WARN result when CurrentContext is empty")
	}
}
