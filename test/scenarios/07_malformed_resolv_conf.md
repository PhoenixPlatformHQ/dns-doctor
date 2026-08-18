# Scenario 07 — Pod with Custom Nameservers (Malformed DNS Config)

## Description
A pod specifies custom nameservers in its `dnsConfig` that point to an external
resolver (e.g. 1.1.1.1). This bypasses CoreDNS entirely, which means in-cluster
service names (e.g. `my-service.default.svc.cluster.local`) will not resolve.

## How to Reproduce

```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: custom-dns-pod
  namespace: default
spec:
  containers:
  - name: busybox
    image: busybox:1.35
    command: ["sleep", "3600"]
  dnsPolicy: None
  dnsConfig:
    nameservers:
    - 1.1.1.1
    searches:
    - external.example.com
EOF

kubectl dns-doctor --namespace default --pod custom-dns-pod
```

## Expected Tool Output (abbreviated)

```
[PASS] Kubernetes API reachable

[INFO] Pod DNS configuration
  Fact:      Pod "custom-dns-pod" has a custom DNS configuration
  Evidence:  dnsPolicy=None  nameservers=[1.1.1.1]  searches=[external.example.com]

[WARN] Pod DNS configuration — custom nameservers
  Fact:      Pod "custom-dns-pod" specifies custom nameservers which may bypass CoreDNS
  Evidence:  nameservers=[1.1.1.1]
  Likely fault domain: Custom nameservers override the cluster DNS service; in-cluster service names may not resolve
  Next check: kubectl get pod -n default custom-dns-pod -o jsonpath='{.spec.dnsConfig}'
```

## What the Tool Must NOT Say

- `[FAIL]` for the custom nameserver check — this is a WARN (intentional config, not a failure)
- That in-cluster DNS "is broken" — it may be intentional for this pod
- That it will "fix" the DNS config or apply any changes

## Summary
`Summary: N PASS  1+ WARN  0 FAIL`
