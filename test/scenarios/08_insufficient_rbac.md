# Scenario 08 — Insufficient RBAC Permissions

## Description
The user running dns-doctor does not have permission to read one or more
Kubernetes resources (e.g. Pods, ConfigMaps, or NetworkPolicies in kube-system).
The tool must degrade gracefully: skip the affected check with a WARN and
continue all other checks.

## How to Reproduce

```bash
# Create a restrictive ServiceAccount with no kube-system access
kubectl create serviceaccount dns-doctor-restricted -n default
kubectl create rolebinding dns-doctor-restricted \
  --clusterrole=view --serviceaccount=default:dns-doctor-restricted \
  --namespace=default

# Use the restricted SA token (or bind a kubeconfig with limited RBAC)
# Then run:
kubectl dns-doctor --namespace default
```

## Expected Tool Output (abbreviated)

```
[PASS] Kubernetes API reachable
  Fact:      API server responded to version check

[WARN] Kubernetes DNS service discovery
  Fact:      Insufficient permissions to read Services in kube-system — skipping check

[WARN] CoreDNS pod health
  Fact:      Insufficient permissions to read Pods in kube-system — skipping check

[WARN] CoreDNS endpoints availability
  Fact:      Insufficient permissions to read Endpoints in kube-system — skipping check

[WARN] CoreDNS ConfigMap availability
  Fact:      Insufficient permissions to read ConfigMap coredns in kube-system — skipping check

[PASS] NetworkPolicy presence
  Fact:      No NetworkPolicies found in namespace "default"

[WARN] Diagnostic summary
  Fact:      DNS Doctor detected potential issues — 4 WARN
```

## What the Tool Must NOT Say

- `[FAIL]` for a permission-denied error — these must be `[WARN]` with the
  phrase "Insufficient permissions to read <resource> — skipping check"
- That it "cannot run" or exits with a non-zero code due to RBAC
- Any sensitive cluster data from a namespace the user lacks access to

## Critical Requirement
The tool MUST complete all checks it has permission to run. A permission
denial on one check must not abort subsequent checks.

## Summary
`Summary: 1+ PASS  4+ WARN  0 FAIL`
Exit code: 0 (the tool ran successfully, even if some checks were skipped)
