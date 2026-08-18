# dns-doctor

**One command to narrow down Kubernetes DNS problems.**

`kubectl dns-doctor` is a **free, open-source** kubectl plugin that runs a
structured set of read-only checks against your cluster's DNS configuration
and reports what it finds — without guessing, without mutations, and without
sending anything outside your machine.

- **Local-first.** No telemetry. No external API calls. No SaaS. No sign-up.
- **Read-only by default.** No cluster resources are created, modified, or deleted.
- **No `cluster-admin` required.** See [Permissions](#permissions-required).
- **EndpointSlice-first.** Uses `discovery.k8s.io/v1 EndpointSlice` as the
  primary backend-discovery source, with a read-only legacy `core/v1 Endpoints`
  fallback for older clusters.

---

## What it checks

| # | Check |
|---|---|
| 1 | Kubernetes API reachable |
| 2 | Current kubeconfig context and cluster name |
| 3 | DNS service discovery (kube-dns / CoreDNS) in kube-system |
| 4 | DNS service ClusterIP assigned and valid |
| 5 | CoreDNS pod health (Running / Ready) |
| 6 | CoreDNS pod restart counts |
| 7 | CoreDNS endpoints availability (EndpointSlice primary, legacy Endpoints fallback) |
| 8 | CoreDNS ConfigMap presence |
| 9 | Obvious CoreDNS Corefile configuration warnings (missing `forward`, missing `loop`) |
| 10 | NetworkPolicy presence in the target namespace |
| 11 | Service endpoint consistency (EndpointSlice primary, legacy Endpoints fallback) |
| 12 | Pod DNS configuration (`dnsPolicy`, custom `dnsConfig`) — no exec required |
| 13 | Actionable next-step summary |

Each check reports a **status** (`PASS` / `WARN` / `FAIL` / `INFO`), the
observed **fact**, the specific **evidence**, a **likely fault domain** where
relevant, and a **next check** command to run.

The tool deliberately does not claim a definitive root cause where evidence is
only suggestive. For example:

```
[WARN] NetworkPolicy presence
  Fact:      NetworkPolicies are present in namespace "production" and may
             affect DNS egress — validate UDP/TCP port 53 access
  Evidence:  2 NetworkPolicy object(s) found: [deny-egress, restrict-dns]
  Next check: kubectl get networkpolicy -n production -o yaml
```

### EndpointSlice and legacy Endpoints fallback

Checks 7 and 11 use `discovery.k8s.io/v1 EndpointSlice` as the primary
endpoint source. This avoids the deprecation warning emitted by Kubernetes
≥ v1.33 when using the legacy `core/v1 Endpoints` API.

If zero EndpointSlices are found for a service (clusters pre-v1.21 or where
the controller has not yet synced), the tool falls back automatically to the
legacy `core/v1 Endpoints` API. The output fact indicates which path was used.

If EndpointSlice access returns `Forbidden`, the check is marked unavailable
with `[WARN]`. The legacy fallback is **not** attempted in the Forbidden case
to avoid a false "no endpoints" result via a different API path.

---

## What it does NOT check

- Live DNS resolution from inside the cluster (requires `--probe`, not yet implemented)
- RBAC or security policy correctness
- Ingress, egress, or service mesh configuration
- External DNS providers
- Custom CNI-specific DNS behaviour
- Multi-cluster or federation setups
- Historical data or trends

---

## Tested environment

Tested against **upstream Kubernetes v1.34** using **Kind v0.30.0**.

The following distributions have **not yet been validated** and may produce
false results due to non-standard CoreDNS pod labels or deployment patterns:

- k3s
- RKE2
- Rancher
- EKS, GKE, AKS, and other managed distributions

Validation of these distributions is tracked as a post-v0.1.0 backlog item.

---

## Privacy and security

**Local-first.** dns-doctor reads from your cluster via the Kubernetes API
using your existing kubeconfig credentials. No data is transmitted to any
external service, analytics platform, or Phoenix infrastructure. Nothing is
stored on disk. The tool never mutates cluster state in default mode.

The `--probe` flag (planned, not yet implemented) would create a temporary
pod for active DNS testing. When implemented, it will explain exactly what
it creates and ensure automatic cleanup. It will never be the default mode.

---

## Permissions required

dns-doctor does not require `cluster-admin`. It needs read access to the
following resources:

| API group | Resource | Verbs | Namespace | Notes |
|---|---|---|---|---|
| *(core)* | `services` | `list`, `get` | `kube-system` + target namespace | |
| `discovery.k8s.io` | `endpointslices` | `list` | `kube-system` + target namespace | Primary endpoint source |
| *(core)* | `endpoints` | `get` | `kube-system` + target namespace | Legacy fallback only |
| *(core)* | `pods` | `list`, `get` | `kube-system` + target namespace | |
| *(core)* | `configmaps` | `get` | `kube-system` | CoreDNS Corefile only |
| `networking.k8s.io` | `networkpolicies` | `list` | target namespace | |

If a permission is missing, the affected check is skipped with a `[WARN]`
message. All other checks continue.

---

## Install

### From source (requires Go 1.21+)

```bash
git clone https://github.com/phoenix-platform/dns-doctor
cd dns-doctor
make build
# Move the binary somewhere on your PATH with the kubectl plugin prefix:
mv kubectl-dns_doctor /usr/local/bin/kubectl-dns_doctor
```

### Pre-built binary

Download the binary for your platform from the
[GitHub Releases page](https://github.com/phoenix-platform/dns-doctor/releases).

Rename it `kubectl-dns_doctor` (or `kubectl-dns_doctor.exe` on Windows) and
place it on your `PATH`. kubectl discovers plugins by prefix: any executable
named `kubectl-<name>` on the PATH is automatically available as
`kubectl <name>`.

Verify kubectl sees the plugin:

```bash
kubectl plugin list
# Expected: /usr/local/bin/kubectl-dns_doctor
```

### Krew (planned)

```bash
kubectl krew install dns-doctor
```

*Krew submission is planned after the public release binaries are available.
Not yet published to the Krew index.*

---

## Usage

```bash
# Run against the default namespace
kubectl dns-doctor

# Run against a specific namespace
kubectl dns-doctor --namespace production

# Run against a specific namespace and inspect a pod's DNS config
kubectl dns-doctor --namespace production --pod my-app-6d8f9-xkp2t

# Output as JSON
kubectl dns-doctor --output json

# Use a specific kubeconfig or context
kubectl dns-doctor --kubeconfig ~/.kube/other-config
kubectl dns-doctor --context staging

# See planned --probe description (not yet implemented)
kubectl dns-doctor --probe
```

### Flags

| Flag | Default | Description |
|---|---|---|
| `--namespace`, `-n` | `default` | Namespace to inspect |
| `--pod` | *(none)* | Pod name to inspect DNS config for |
| `--output`, `-o` | `human` | Output format: `human` or `json` |
| `--probe` | false | Active DNS probe (not yet implemented — prints description) |
| `--kubeconfig` | `$KUBECONFIG` or `~/.kube/config` | Path to kubeconfig file |
| `--context` | current-context | kubeconfig context to use |

---

## Example outputs

### Healthy cluster

```
────────────────────────────────────────────────────────────
  kubectl dns-doctor
────────────────────────────────────────────────────────────
[PASS] Kubernetes API reachable
  Fact:      API server responded to version check
  Evidence:  Server version v1.34.0

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
  Fact:      Service "kube-dns" has 2 ready endpoint(s) across 1 EndpointSlice(s)
  Evidence:  readyEndpoints=2  totalEndpoints=2  slices=1

[PASS] CoreDNS ConfigMap availability
  Fact:      ConfigMap coredns found in kube-system
  Evidence:  configmap/coredns  keys=[Corefile]

[PASS] CoreDNS Corefile configuration
  Fact:      No obvious configuration warnings detected in Corefile
  Evidence:  forward and loop directives are present

[PASS] NetworkPolicy presence
  Fact:      No NetworkPolicies found in namespace "default"
  Evidence:  NetworkPolicy list returned 0 items

[PASS] Diagnostic summary
  Fact:      All checks passed — no DNS anomalies detected
  Evidence:  0 FAIL  0 WARN

Summary: 10 PASS  0 WARN  0 FAIL  1 INFO
```

### Degraded cluster (CoreDNS crashing)

See [`docs/example-output-degraded.txt`](docs/example-output-degraded.txt).

---

## Release integrity

Every release includes:

- **`checksums.txt`** — SHA256 hashes of all release archives. Verify with:
  ```bash
  sha256sum --check checksums.txt
  ```

- **GitHub Artifact Attestations** — build provenance for every release
  archive, produced automatically by the release workflow using
  [GitHub Artifact Attestations](https://docs.github.com/en/actions/security-guides/using-artifact-attestations-to-establish-provenance-for-builds).
  Verify with:
  ```bash
  gh attestation verify dns-doctor_0.1.0_linux_amd64.tar.gz \
    --owner phoenix-platform
  ```

Binaries are **not** GPG-signed or Cosign-signed in this release.

---

## Build

```bash
# Current platform
make build

# All platforms (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64)
make build-all

# Run tests
make test

# Lint (requires golangci-lint)
make lint
```

---

## Known limitations

1. **`--probe` not implemented.** The flag is accepted and describes its
   planned behaviour but performs no cluster action.
2. **EndpointSlice Forbidden path.** When `endpointslices` access is denied,
   the check emits `[WARN]`. Full simulation via unit test requires an httptest
   mock server (known gap — current test uses NotFound as proxy).
3. **CoreDNS pod label assumption.** Pods are located by label
   `k8s-app=kube-dns`. Distributions that label CoreDNS differently (e.g.,
   some Rancher distros) may return a false "no pods found" result.
4. **Corefile parsing is text-based.** Check 9 scans for keyword presence only
   (`forward`, `loop`). Reformatted or complex Corefiles could produce false
   negatives.
5. **NetworkPolicy rules not evaluated.** Check 10 flags presence only.
   Whether a specific policy blocks DNS port 53 is CNI-specific and out of scope.
6. **Not validated on k3s, RKE2, or managed distributions.** See
   [Tested environment](#tested-environment).

---
