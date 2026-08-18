# Scenario 05 — No Endpoints on a Service

## Description
A Service exists in the target namespace but has no Ready pods backing it.
DNS resolves the Service name, but connections fail because there are no endpoints.
This is a common cause of "DNS works but service is unreachable" reports.

## How to Reproduce

```bash
kubectl apply -f demo/manifests/no-endpoints-service.yaml
kubectl dns-doctor --namespace test-no-endpoints
```

## Expected Tool Output (abbreviated)

```
[PASS] Kubernetes API reachable

[PASS] Kubernetes DNS service discovery

[PASS] CoreDNS pod health

[WARN] Service test-no-endpoints/broken-service endpoint consistency
  Fact:      Service "broken-service" has no ready endpoints — DNS will resolve but connections will fail
  Evidence:  Endpoints object contains zero ready addresses
  Likely fault domain: No pods are Ready and selected by this Service — TCP connections will be refused after DNS resolution
  Next check: kubectl get endpoints -n test-no-endpoints broken-service

[WARN] Diagnostic summary
  Fact:      DNS Doctor detected potential issues — 1 WARN
```

## What the Tool Must NOT Say

- `[FAIL] DNS is broken` — DNS itself is fine; the issue is at the service layer
- That the service "blocks DNS" — DNS resolution succeeds; the endpoint is empty
- Any claim about what the application is doing

## Summary
`Summary: N PASS  1+ WARN  0 FAIL`

## Note
DNS itself is healthy in this scenario. The WARN correctly distinguishes
"DNS resolves" from "connection succeeds". This is an important nuance the
tool must preserve.
