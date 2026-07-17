# pack-testbed

> [!WARNING]
> **This is a CI test fixture, not a product.** Do not install it on a shared or
> production cluster. It exists only to exercise the Nebari reusable workflows and
> the NebariApp CRD API in ephemeral test clusters.

A dedicated, real-but-non-product Nebari pack that serves as the standing
integration fixture for the reusable workflows in
[`nebari-dev/.github`](https://github.com/nebari-dev/.github) and for the
NebariApp CRD API.

## What it does

1. **Supply-chain fixture.** Calls `pack-build-image` + `pack-release`, so a
   release produces a signed + SLSA-provenance-attested + SBOM-attested image and
   chart. This is the real end-to-end exercise of the signing workflows at a pack
   release, and it produces real artifacts that `provenance-collector-pack` can
   verify.
2. **NebariApp CRD harness.** Driven by
   [`action-nebari-sandbox`](https://github.com/nebari-dev/action-nebari-sandbox)
   (`platform` profile, which ships the nebari-operator + gateway + cert-manager +
   Keycloak) to exercise and validate the NebariApp CRD API in two tiers:
   conformance (spec matrix + negative cases -> conditions/reasons/resources) and
   functional (real traffic + OIDC through the gateway).

## Security model

The two footguns - an app that reflects credentials, and a publishable test
chart someone deploys to production - are designed out:

- **The app never returns raw secret material.** `/whoami` returns only sanitized
  derived facts (`bearer_present`, `groups`, `sub`, ...), never raw
  `Authorization`/`Cookie` values or JWTs. A raw-echo debug mode exists only when
  `TESTBED_UNSAFE_ECHO=1` (chart value `unsafeEcho: true`), which the ephemeral
  test harness may set - never in a shipped default. The introspection route is
  auth-gated; only `/healthz` is public.
- **Not an installable product.** Not listed in the pack catalog. The chart is
  never published to `quay.io/nebari/charts` (the release sync PR is closed, not
  merged, pending the `sync: false` opt-out in nebari-dev/.github#47).
- **Cluster tests run only in ephemeral k3d sandboxes** with no production
  credentials in scope, torn down after each run.
- Minimal, nonroot, read-only-rootfs, distroless image; keyless (secretless) CI
  signing; least-privilege job permissions.

## Layout

```
app/                    # Go echo/introspection service + Dockerfile (distroless/static:nonroot)
chart/                  # Helm chart, NebariApp templated, image.tag pinnable
tests/
  conformance/          # NebariApp spec permutations + expected outcomes
  functional/           # traffic/auth/TLS assertions via gateway + Keycloak
.github/workflows/      # build-images, release, verify-artifacts, crd-conformance, crd-functional
docs/design.md          # full design
```

## Status

Scaffold (P0). Supply-chain callers are wired; `verify-artifacts` and the two CRD
tiers are stubs pending the signing-enabled reusable-workflow ref
(nebari-dev/.github PR #46) and the assertion implementations. See
[docs/design.md](docs/design.md) for the phased plan.

## Developing

```bash
# App
cd app && go test ./...

# Chart
helm lint chart
helm template t chart --set nebariapp.enabled=true --set nebariapp.hostname=pack-testbed.nebari.local
```
