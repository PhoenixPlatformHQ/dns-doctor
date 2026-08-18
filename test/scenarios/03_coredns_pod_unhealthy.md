# Scenario 03 — CoreDNS Pod Unhealthy (CrashLoopBackOff)

## Description
CoreDNS pods exist but are in CrashLoopBackOff with a high restart count.
DNS resolution is intermittently failing or completely broken.

## How to Reproduce

```bash
# Apply a broken Corefile to trigger a CoreDNS crash loop
kubectl apply -f demo/manifests/broken-coredns-configmap.yaml
# Wait ~30s for pods to restart
kubectl dns-doctor
# Restore with:
kubectl delete configmap coredns -n kube-system
kubectl rollout restart deployment coredns -n kube-system
```

## Expected Tool Output (abbreviated)

```
[PASS] Kubernetes API reachable

[PASS] Kubernetes DNS service discovery

[PASS] DNS service ClusterIP

[FAIL] CoreDNS pod health
  Fact:      CoreDNS pod coredns-xyz is not Running/Ready
  Evidence:  phase=Failed ready=false
  Likely fault domain: CoreDNS pod may be crashing, pending, or evicted
  Next check: kubectl describe pod -n kube-system coredns-xyz

[FAIL] CoreDNS pod coredns-xyz restart count
  Fact:      CoreDNS pod coredns-xyz has elevated restart count (12)
  Evidence:  restartCount=12 container=coredns
  Likely fault domain: Repeated CoreDNS crashes — DNS resolution is likely degraded
  Next check: kubectl logs -n kube-system coredns-xyz --previous

[FAIL] CoreDNS endpoints availability
  Fact:      DNS service/kube-dns has no ready endpoints

[WARN] CoreDNS Corefile configuration
  Fact:      No 'forward' directive found in Corefile

[FAIL] Diagnostic summary
```

## What the Tool Must NOT Say

- `[PASS] CoreDNS pod health` for a crashing pod
- That DNS "is definitely broken" — only that it "is likely degraded"
- That the Corefile issue "caused" the crash — only flag the observation

## Summary
`Summary: N PASS  1+ WARN  3+ FAIL`
