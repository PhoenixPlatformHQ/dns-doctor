# Scenario 01 — Healthy CoreDNS

## Description
A fully healthy cluster where CoreDNS is running normally, has valid endpoints,
and all DNS checks pass.

## How to Reproduce

```bash
# Standard cluster (e.g. kind, minikube, EKS, GKE)
kubectl get pods -n kube-system -l k8s-app=kube-dns
# Expected: 2 pods in Running state with 0/few restarts

kubectl dns-doctor
```

## Expected Tool Output (abbreviated)

```
[PASS] Kubernetes API reachable
  Fact:      API server responded to version check
  Evidence:  Server version v1.28.4

[INFO] Kubernetes context and cluster
  Fact:      Active kubeconfig context and cluster resolved
  Evidence:  context=kind-kind  cluster=kind-kind

[PASS] Kubernetes DNS service discovery
  Fact:      DNS service found in kube-system
  Evidence:  service/kube-dns ClusterIP=10.96.0.10

[PASS] DNS service ClusterIP
  Fact:      DNS service has a valid ClusterIP
  Evidence:  ClusterIP=10.96.0.10

[PASS] CoreDNS pod health
  Fact:      CoreDNS pod coredns-5d78c9869d-abc12 is Running and Ready
  Evidence:  phase=Running ready=true

[PASS] CoreDNS pod coredns-5d78c9869d-abc12 restart count
  Fact:      CoreDNS pod restart count is zero
  Evidence:  restartCount=0 container=coredns

[PASS] CoreDNS endpoints availability
  Fact:      DNS service/kube-dns has 2 ready endpoint(s)
  Evidence:  readyAddresses=2

[PASS] CoreDNS ConfigMap availability
  Fact:      ConfigMap coredns found in kube-system

[PASS] CoreDNS Corefile configuration
  Fact:      No obvious configuration warnings detected in Corefile
  Evidence:  forward and loop directives are present

[PASS] NetworkPolicy presence
  Fact:      No NetworkPolicies found in namespace "default"

[PASS] Diagnostic summary
  Fact:      All checks passed — no DNS anomalies detected
```

## What the Tool Must NOT Say

- `[FAIL]` on any check in a healthy cluster
- Any claim about DNS being "broken" or "blocked"
- Any mutation or "fixing" of cluster state

## Summary
`Summary: 9+ PASS  0 WARN  0 FAIL`
