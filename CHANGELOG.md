# Changelog

All notable changes to dns-doctor are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).
This project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Planned
- `--probe` flag: active in-cluster DNS resolution test via a temporary pod
- EndpointSlice Forbidden path: full simulation via httptest mock server
- k3s compatibility validation
- RKE2 compatibility validation

---

## [0.1.0] — 2026-08-18

First public release.

### Added
- Check 1: Kubernetes API reachable
- Check 2: Current kubeconfig context and cluster name
- Check 3: DNS service discovery (kube-dns / CoreDNS) in kube-system
- Check 4: DNS service ClusterIP assigned and valid
- Check 5: CoreDNS pod health (Running / Ready)
- Check 6: CoreDNS pod restart counts (`[WARN]` at 1–4, `[FAIL]` at 5+)
- Check 7: CoreDNS endpoints availability (EndpointSlice primary, legacy Endpoints fallback)
- Check 8: CoreDNS ConfigMap presence
- Check 9: Corefile configuration warnings (missing `forward`, missing `loop`)
- Check 10: NetworkPolicy presence in the target namespace
- Check 11: Service endpoint consistency (EndpointSlice primary, legacy Endpoints fallback)
- Check 12: Pod DNS configuration from pod spec — no exec required
- Check 13: Actionable next-step summary
- Human-readable output with `[PASS]` / `[WARN]` / `[FAIL]` / `[INFO]` per check
- JSON output via `--output json`
- `--namespace`, `--pod`, `--kubeconfig`, `--context` flags
- `--probe` flag stub (describes planned behaviour; not yet functional)
- Graceful RBAC degradation: missing permissions emit `[WARN]` and continue
- `discovery.k8s.io/v1 EndpointSlice` as primary endpoint source
- Legacy `core/v1 Endpoints` fallback for clusters pre-dating EndpointSlice
- Endpoint ready-state: `Conditions.Ready == nil` treated as ready per KEP-752
- 35 unit tests (fake client — no real cluster required for `go test`)
- Cross-platform builds: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64
- GitHub Actions CI workflow (build + vet + test + race detector on every push/PR)
- GitHub Actions release workflow (GoReleaser + GitHub Artifact Attestations on tag)
- SHA256 checksums file (`checksums.txt`) included in every release
- Krew plugin manifest stub (`krew/dns-doctor.yaml`) — SHA256s filled post-release

### Security
- No cluster mutations in default mode (all API calls are GET/LIST)
- No telemetry, no external network calls, no secret collection
- No `cluster-admin` required
- Statically compiled binary (`CGO_ENABLED=0`)

---

[Unreleased]: https://github.com/phoenix-platform/dns-doctor/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/phoenix-platform/dns-doctor/releases/tag/v0.1.0
