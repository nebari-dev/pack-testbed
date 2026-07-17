# NebariApp functional suite (P3)

End-to-end assertions on an ephemeral sandbox `platform` cluster: deploy this
pack with an auth-enabled NebariApp and drive real traffic + OIDC through the
gateway, using the echo app's sanitized `/whoami` output to confirm behavior.

Driven by `crd-functional.yaml`. Uses sandbox outputs: `gateway-ip`,
`keycloak-realm-admin-password`, `keycloak-issuer-url`, `ca-cert-path`.

## Assertions

| # | Scenario | Expect |
|---|----------|--------|
| 1 | GET `/healthz` (public route) | 200, no auth |
| 2 | GET protected route, unauthenticated | 302 redirect to Keycloak |
| 3 | Complete a real OIDC login, then GET protected route | 200; reaches the app |
| 4 | forwardAccessToken=true | `/whoami` reports `bearer_present` + `forwarded_by_gateway` + expected `groups` (no raw token) |
| 5 | User not in required group | 403 |
| 6 | TLS | gateway serves a valid cert (verified with `ca-cert-path`) |

TODO: implement the login helper (Keycloak realm admin creds) and the curl/assert
harness; keep all assertions on sanitized `/whoami` fields (never raw tokens).
