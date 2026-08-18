# Scenario 04 — DNS Service Missing

## Description
The kube-dns Service has been accidentally deleted from kube-system.
All pods using ClusterDNS will fail to resolve any name.

## How to Reproduce

```bash
kubectl delete svc kube-dns -n kube-system
kubectl dns-doctor
# Restore: redeploy CoreDNS or apply the kube-dns service manifest
```

## Expected Tool Output (abbreviated)

```
[PASS] Kubernetes API reachable

[FAIL] Kubernetes DNS service discovery
  Fact:      No DNS service found in kube-system with expected labels or name
  Evidence:  Tried labels k8s-app=kube-dns, app=coredns, and GET kube-dns
  Likely fault domain: CoreDNS or kube-dns may not be deployed
  Next check: kubectl get svc -n kube-system

[INFO] CoreDNS checks skipped
  Fact:      Remaining CoreDNS checks skipped because DNS service was not found
  Evidence:  checks 4–9 require a resolvable DNS service
  Next check: Ensure kube-dns/CoreDNS is deployed in kube-system

[FAIL] Diagnostic summary
  Fact:      DNS Doctor detected potential issues — 1 FAIL
```

## What the Tool Must NOT Say

- That it checked ClusterIP, pods, or endpoints (those checks are skipped when service is missing)
- That "DNS is broken" — only that the service is absent and checks are skipped

## Summary
`Summary: 1 PASS  0 WARN  2 FAIL`
