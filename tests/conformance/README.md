# NebariApp conformance matrix (P2)

Each case is a NebariApp manifest applied to an ephemeral sandbox `platform`
cluster (operator + gateway + cert-manager + Keycloak present), with an expected
outcome asserted by `crd-conformance.yaml`.

## Positive matrix (assert conditions + created resources)

| Case | Spec highlights | Expect |
|------|-----------------|--------|
| routing-basic | routing.routes=[/] | `RoutingReady`; HTTPRoute created |
| routing-omitted | no `routing` | no HTTPRoute; routing/TLS considered disabled |
| tls-on | routing.tls.enabled=true | `TLSReady`; Certificate created |
| tls-off | routing.tls.enabled=false | HTTP-only listener; no Certificate |
| public-routes | routing.publicRoutes=[/healthz] | separate unprotected HTTPRoute for /healthz |
| auth-gateway | auth.enabled, enforceAtGateway=true | `AuthReady`; SecurityPolicy + `<name>-oidc-client` Secret |
| auth-app-native | auth.enabled, enforceAtGateway=false | OIDC Secret created; NO SecurityPolicy |
| auth-groups | auth.groups=[admin] | SecurityPolicy restricts to group |
| spa-client | auth.spaClient.enabled | public SPA client id in Secret |
| device-flow | auth.deviceFlowClient.enabled | device client id in Secret |
| gateway-internal | gateway=internal | routed via the internal Gateway |
| landing-page | landingPage.enabled | `status.serviceDiscovery` populated |

## Negative matrix (assert failure reason)

| Case | Trigger | Expect reason |
|------|---------|---------------|
| ns-not-opted-in | namespace missing `nebari.dev/managed=true` | `NamespaceNotOptedIn` |
| service-missing | service.name points at nonexistent Service | `ServiceNotFound` |
| bad-hostname | hostname violates the CRD pattern | CRD schema rejection at apply |
| fwd-without-enforce | forwardAccessToken=true, enforceAtGateway=false | validation rejection |
| hostname-conflict | two NebariApps, same hostname | `GatewayListenerConflict` |

TODO: add one manifest per case and the assertion harness.
