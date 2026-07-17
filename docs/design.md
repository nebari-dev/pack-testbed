# pack-testbed: reusable-workflow + NebariApp CRD integration test bed

- **Date:** 2026-07-17
- **Proposed repo:** `nebari-dev/pack-testbed` (new, dedicated)
- **Status:** Design (brainstorm output, pending review)

## Goal

A dedicated, real (but non-product) Nebari pack that serves as the org's standing
integration fixture for the reusable workflows in `nebari-dev/.github` and for the
NebariApp CRD API. It has two roles:

1. **Supply-chain fixture.** Calls `pack-build-image` + `pack-release`, so a release
   produces signed + SLSA-provenance-attested + SBOM-attested image and chart. This
   is the "real pack release" end-to-end exercise the signing work (nebari-dev/.github
   PR #46) otherwise lacks, and it produces real artifacts that
   `provenance-collector-pack` can verify (producer -> consumer chain).
2. **NebariApp CRD harness.** Driven by `action-nebari-sandbox` to exercise and
   validate the NebariApp CRD API, in two tiers (conformance + functional).

Non-goal: being an installable product. It must never be a catalog pack, and must
never make a real cluster vulnerable (see Security model).

## Security model (primary constraint)

The two footguns are (a) an app that reflects headers/tokens, and (b) a publishable
test chart someone deploys to production. Designed out as follows:

1. **App never returns secret material - safe by default.** The introspection
   endpoint returns only sanitized, derived facts for assertions
   (`{"bearer_present": true, "session_cookie_present": false, "groups": ["admin"],
   "sub": "...", "forwarded_by_gateway": true}`). It never echoes raw
   `Authorization`/`Cookie` values or the raw JWT. Any raw-echo debug mode is behind
   an explicit env var (`TESTBED_UNSAFE_ECHO=1`) that only the ephemeral test harness
   sets - never in the shipped default. The introspection route is auth-required and
   non-public; only `/healthz` is public.
2. **Never an installable product.** Not listed in the software-pack catalog/dashboard.
   Chart `NOTES.txt` + README banner state loudly it is a CI fixture, not for
   production. The `pack-release` run still executes (that is the test), but the chart
   does NOT land in `quay.io/nebari/charts`: the sync PR to `helm-repository` is
   auto-closed, and verification uses the GitHub Release artifact + cosign bundle. The
   image publishes to a clearly-named test path, not a product namespace.
3. **Cluster tests only in ephemeral sandboxes.** Conformance + functional tiers run
   against a fresh `action-nebari-sandbox` k3d cluster per run, torn down after. The
   workflows have no production kubeconfig/credentials in scope, so they cannot target
   a real cluster. Provisioned OIDC clients / SecurityPolicies / Keycloak groups die
   with the sandbox.
4. **Least-privilege CI.** Keyless OIDC signing (no long-lived secrets). Per-job
   permissions scoped to the minimum (`id-token`/`attestations`/`packages: write`
   only where needed). Fork PRs build without pushing or secrets. Minimal pinned
   dependencies; the SBOM (+ optional scan) surfaces CVEs on the fixture image.

## Decisions (locked)

- Repo: `nebari-dev/pack-testbed` (dedicated, mirrors `action-nebari-sandbox-test`).
- App: Go static binary on `gcr.io/distroless/static:nonroot` - minimal attack
  surface, nonroot, read-only rootfs, no outbound calls (decodes the forwarded token
  payload only), rich SBOM via Go buildinfo.
- CRD validation: tiered (conformance PR-gate + functional on a heavier trigger),
  both on the sandbox `platform` profile (operator + full stack already present).
- Reusable-workflow refs: target the released/tagged ref (no branch-racing; #46 is
  validated separately via llm-serving-pack).

## Repository layout

```
app/                    # Go echo/introspection service + Dockerfile (distroless/static:nonroot)
chart/                  # Helm chart (real name), NebariApp templated, image.tag pinnable
tests/
  conformance/          # NebariApp spec permutations + expected-outcome tables
  functional/           # scripts: traffic/auth/TLS assertions via gateway + Keycloak
.github/workflows/
  build-images.yaml     # -> pack-build-image@<signing ref>
  release.yaml          # -> pack-release@<signing ref>
  verify-artifacts.yaml # cosign verify + verify-attestation (provenance+SBOM); negative cases
  crd-conformance.yaml  # sandbox cluster-only + pinned operator; matrix; assert conditions/resources
  crd-functional.yaml   # sandbox platform; real traffic + OIDC; heavier trigger
```

## The echo/introspection app (Go)

- `GET /healthz` -> 200, unauthenticated (target for `publicRoutes`).
- `GET /whoami` -> auth-required; returns sanitized derived facts only (see Security
  model): presence booleans for bearer/session cookie, decoded non-secret claims
  (`groups`, `sub`, `preferred_username`), and `forwarded_by_gateway`. Never raw
  tokens/headers.
- `livez`/`readyz` probes.
- Decodes (does not verify) the forwarded token payload - the gateway already
  enforced - so no JWKS/outbound calls, no CA certs needed.

## Supply-chain role (Phase 1)

- `build-images.yaml` -> `pack-build-image@<ref with sign-oci>`: build + sign + attest
  (provenance + SBOM) the image on GHCR & Quay (test path).
- `release.yaml` -> `pack-release@<ref with sign-blob>`: pin image tag, package + sign
  + attest the chart `.tgz`, GitHub Release; sync PR to `helm-repository` is opened
  (validates the workflow) then auto-closed (never lands in the shared catalog).
- `verify-artifacts.yaml` (the actual test): `cosign verify` (assert the expected
  workflow OIDC identity) + `cosign verify-attestation` for the SLSA-provenance and
  SPDX-SBOM predicates, on the released image (by digest) and the chart bundle; plus
  negative cases (tampered blob, wrong identity) that must fail closed.
- Caller jobs set `id-token: write` + `attestations: write` (the reusable-workflow
  permissions gotcha, validated here first).

## NebariApp CRD role (tiered)

Provisioned by the `action-nebari-sandbox` `platform` profile, which deploys the
full foundational stack **including `nebari-operator`** (confirmed: it's a
foundational ArgoCD app in NIC's set, alongside cert-manager, envoy-gateway,
gateway-config, keycloak). Both tiers therefore run on `platform`; we do not
install or version-pin the operator ourselves - it's whatever NIC's foundational
set ships. (Reconciling a NebariApp to Ready genuinely needs the Gateway,
cert-manager, and Keycloak too, so `cluster-only` is insufficient anyway.)

**Conformance tier** (`crd-conformance.yaml`, `platform` profile, PR-gate): a
matrix of NebariApp specs across the API surface - routing present/omitted, `tls`
on/off, auth off / gateway-enforced / app-native / `spaClient` / `deviceFlowClient` /
`groups`, `gateway: public|internal`, `landingPage` on/off - plus negative cases
mapped to documented reasons: unlabeled namespace -> `NamespaceNotOptedIn`; missing
service -> `ServiceNotFound`; bad hostname -> CRD schema rejection;
`forwardAccessToken` without `enforceAtGateway` -> validation rejection; duplicate
hostname -> `GatewayListenerConflict`. For each, assert the
`RoutingReady`/`TLSReady`/`AuthReady`/`Ready` conditions + reasons and the
presence/absence of HTTPRoute, SecurityPolicy, Certificate, `<name>-oidc-client`
Secret, and `status.serviceDiscovery`.

**Functional tier** (`crd-functional.yaml`, heavier trigger, `platform` profile):
deploy the echo app + an auth-enabled NebariApp; via the sandbox `gateway-ip` /
hostnames and Keycloak creds - public route -> 200; protected route unauthenticated
-> 302 to Keycloak; complete a real OIDC login -> reach the app; `forwardAccessToken`
injects a Bearer the echo app confirms (sanitized); wrong-group user -> 403; TLS
served with a valid cert.

## How it grows

Each new reusable workflow in `nebari-dev/.github` gets a caller + a verify job here,
so the repo becomes the org's standing integration suite. A scheduled "canary" run
against the workflows' `@main` catches regressions before they reach real packs.

## Phasing

- P0: scaffold repo + Go echo app + chart.
- P1: supply-chain callers + `verify-artifacts` (targets released signing ref).
- P2: conformance tier (pinned operator on `cluster-only` sandbox).
- P3: functional tier (`platform` sandbox; Keycloak OIDC scripting).

## Open items / upstream asks

- `pack-release@v1` unconditionally opens a shared-repo sync PR; for a frequently
  running fixture that is noise/risk. Filed as nebari-dev/.github#47 (`sync: false`
  input). Until it lands, either auto-close the sync PR or gate `release.yaml` to
  `workflow_dispatch`.
- Operator version: whatever the sandbox/NIC foundational set ships. If we ever need
  to validate against a specific `nebari-operator` version, investigate whether the
  sandbox exposes an override (e.g. via `nic-config`) or run those cases in the
  operator's own repo.

## Definition of done (verification)

- P1: a release produces cosign-verifiable, provenance- and SBOM-attested image +
  chart; `verify-artifacts` passes on positives and fails closed on tamper/
  wrong-identity; chart never appears in `quay.io/nebari/charts`.
- P2: conformance matrix asserts the documented conditions/reasons/resources for every
  spec permutation and negative case, on an ephemeral sandbox.
- P3: functional suite proves enforce/allow/deny/forwarded-token and TLS end-to-end
  against ephemeral Keycloak + gateway.
- Security: shipped image never returns raw secrets; chart is not catalog-installable;
  no workflow can touch a non-ephemeral cluster.
```
