# Scenario 06 — NetworkPolicy Present in Namespace

## Description
One or more NetworkPolicy objects exist in the target namespace. The tool must
flag their presence without claiming they block DNS (which cannot be determined
without CNI-specific rule evaluation).

## How to Reproduce

```bash
kubectl apply -f demo/manifests/networkpolicy-deny-dns.yaml
kubectl dns-doctor --namespace test-netpol
```

## Expected Tool Output (abbreviated)

```
[PASS] Kubernetes API reachable

[PASS] Kubernetes DNS service discovery

[WARN] NetworkPolicy presence
  Fact:      NetworkPolicies are present in namespace "test-netpol" and may affect DNS egress — validate UDP/TCP port 53 access
  Evidence:  1 NetworkPolicy object(s) found: [deny-all-egress]
  Likely fault domain: A NetworkPolicy may be restricting egress to port 53 (UDP/TCP) required for DNS resolution
  Next check: kubectl get networkpolicy -n test-netpol -o yaml   # review egress rules for port 53
```

## What the Tool Must NOT Say

- `[FAIL] NetworkPolicy blocks DNS` — this is a deterministic claim that cannot
  be made without evaluating CNI behaviour
- That the policy "is blocking" anything — only that it "may affect" DNS
- Any claim about specific packet flows

## Verification
The tool output must contain the exact word "may" when describing the
NetworkPolicy's relationship to DNS. It must never say "blocks" in the
context of this check.

## Summary
`Summary: N PASS  1 WARN  0 FAIL`
