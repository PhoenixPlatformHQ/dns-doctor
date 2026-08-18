# Scenario 02 — CoreDNS Unavailable (Deployment Scaled to Zero)

## Description
The CoreDNS Deployment has been scaled to 0 replicas. No pods are running.
DNS resolution in the cluster will fail for all workloads.

## How to Reproduce

```bash
kubectl scale deployment coredns -n kube-system --replicas=0
kubectl dns-doctor
# Restore with:
kubectl scale deployment coredns -n kube-system --replicas=2
```

## Expected Tool Output (abbreviated)

```
[PASS] Kubernetes API reachable
  Fact:      API server responded to version check
  Evidence:  Server version v1.28.4

[PASS] Kubernetes DNS service discovery
  Fact:      DNS service found in kube-system
  Evidence:  service/kube-dns ClusterIP=10.96.0.10

[PASS] DNS service ClusterIP
  Fact:      DNS service has a valid ClusterIP

[FAIL] CoreDNS pod health
  Fact:      No CoreDNS pods found in kube-system with label k8s-app=kube-dns
  Evidence:  pod list returned 0 items
  Likely fault domain: CoreDNS Deployment may be scaled to zero or incorrectly labelled
  Next check: kubectl get pods -n kube-system -l k8s-app=kube-dns

[FAIL] CoreDNS endpoints availability
  Fact:      DNS service/kube-dns has no ready endpoints
  Evidence:  Subsets contain zero ready addresses
  Likely fault domain: All CoreDNS pods may be unhealthy or not yet scheduled
  Next check: kubectl get endpoints -n kube-system kube-dns

[FAIL] Diagnostic summary
  Fact:      DNS Doctor detected potential issues — 2 FAIL
```

## What the Tool Must NOT Say

- `[PASS] CoreDNS pod health` — pods are not running
- That DNS "is broken" — it should say DNS "may fail" or "is likely degraded"
- That it has fixed or restarted anything

## Summary
`Summary: N PASS  0 WARN  2+ FAIL`
