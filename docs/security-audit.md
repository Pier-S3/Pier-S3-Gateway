# Security audit & penetration-vector analysis

_Date: 2026-06-11. Scope: `internal/`, `cmd/`, `deployments/`, the React SPA._

Pier was reviewed as an attacker would approach it: an OIDC-authenticated reverse
proxy in front of S3-compatible storage, with a browser SPA. This document lists
every finding by severity, marks which were **fixed in this pass** and which are
**recommended** follow-ups, and ends with the controls that are already strong.

## Fixed in this pass

| ID | Severity | Issue | Fix |
|----|----------|-------|-----|
| RL-1 | **Critical** | Rate limiter keyed on the **left-most** `X-Forwarded-For` entry, which any client can forge - so the per-IP limit on the OIDC endpoints was trivially bypassable by rotating the header. | `clientIP` now uses the **right-most** (proxy-appended) hop; tests updated. (`internal/webui/ratelimit.go`) |
| ACL-1 | **High** | Multipart-upload admin (`?uploads`, `?uploadId`) was not blocked - a read-only user could list other users' in-progress uploads; `POST` at bucket level was passed through. | Added `uploads`/`uploadId`/`partNumber` to the admin blocklist and block bucket-level `POST`. (`internal/acl/resolver.go`) |
| ACL-2 | **Medium** | Admin sub-resource match was **case-sensitive**: `?ACL` or `?Versioning` could slip past the `acl`/`versioning` blocklist on a lenient backend. | Matching now lowercases the query key before lookup; all keys normalised. |
| PRX-1 | **High** | The S3 proxy's `GET /` bucket listing ignored wildcard (`*-<policy>`) grants, so a `*-ro` auditor saw an empty list (inconsistent with the UI API, masking real access). | `handleListBuckets` now expands wildcard grants. (`internal/proxy/s3handler.go`) |
| LOG-1 | **Medium** | Access-denied logs included the caller's full group set (their entire authorization profile) - sensitive if logs ship to a lower-trust aggregator. | Log `group_count` instead of the group list. |
| HDR-1 | **Medium** | S3 proxy error responses lacked `X-Content-Type-Options: nosniff`. | Added to `writeJSONError`. |

### Follow-up pass (applied)

| ID | Severity | Issue | Fix |
|----|----------|-------|-----|
| CSRF-1 | **High** | UI API accepted the `access_token` cookie for state-changing `PUT`/`DELETE` with no server-side CSRF defense. | AuthMiddleware rejects cookie-authenticated, cross-site state changes via Fetch-Metadata (`Sec-Fetch-Site`) + Origin/Host check; Bearer-auth and safe methods are exempt. Tested. (`internal/webui/auth.go`, `csrf_test.go`) |
| SSRF-1 | **High** | `OIDC_ISSUER`/`OIDC_JWKS_URL`/`OIDC_DISCOVERY_URL` were fetched with no scheme validation - a plaintext or attacker-influenced URL could MITM the trust anchor. | `ResolveOIDC` now requires `https` (discovery URL validated **before** fetch); `http` only with explicit `OIDC_ALLOW_INSECURE=true`; non-http(s) schemes always rejected. Tested. (`internal/config/config.go`) |
| NETPOL-1 | **High** | Raw + Helm NetworkPolicy egress used `namespaceSelector: {}` (any namespace) on 443/8333. | Raw manifest scoped to named Keycloak/SeaweedFS namespace+pod selectors; Helm egress made configurable (namespace/pod labels or `ipBlock`) with a rendered WARNING when left unscoped. |
| DKR-1 | **High** | Dockerfile used `go mod download \|\| true` (swallowed failures) and `npm install`. | `go mod download` (no `\|\| true`), `go build -mod=readonly`, `npm ci` (lockfile-reproducible), digest-pinning guidance; SeaweedFS pinned off `:latest`; dev creds marked DEV-ONLY. |

### Hardening pass (applied 2026-06-11)

The previously "recommended" items below were applied in this pass:

| ID | Severity | Issue | Fix |
|----|----------|-------|-----|
| DISC-1 | **Low** | Discovered issuer was not cross-checked against `OIDC_ISSUER`. | `ResolveOIDC` now fails closed when discovery is consulted and the discovered issuer differs from the configured one (trailing slash ignored). Tested. (`internal/config/config.go`) |
| CK-1 | **Medium** | `access_token`/`refresh_token` (and transient login) cookies used `Path=/`, so they rode along on static-asset requests. | All auth cookies are scoped to `auth.CookiePath` (`/api/`); every consumer lives under that prefix and all `clearCookie`/`ClearRefreshTokenCookie` calls use the same path so logout keeps working. Tested. |
| LOG-2 | **Medium** | Token-endpoint error bodies were interpolated into error strings logged at `Error` level. | Errors now carry only the status + parsed OAuth2 `error` code (`oauthErrorCode`); raw bodies (incl. `error_description`/HTML) are never propagated. Tested. (`internal/auth/oidc.go`) |
| EDGE-1 | **Medium** | `/_health` `/_ready` were reachable through the public k8s Ingress (NGINX config already blocked them). | Probe endpoints moved to a dedicated listener (`LISTEN_HEALTH_ADDR`, default `:8082`) that the Service/Ingress never expose; kubelet probes target the `health` containerPort directly. A `server-snippet` 404 was considered first but rejected: it is Critical-risk and refused by ingress-nginx >= 1.12 at the default `annotations-risk-level: High`. |
| RED-1 | **Low** | No explicit `redirect_uri` on the server-side login flow. | New `OIDC_REDIRECT_URI` env var wired through `NewOIDCConfig`; allowlist the exact value at the IdP. Empty keeps the SPA-supplied behavior. |
| HELM-1 | **Low** | An all-empty `secret.data` rendered an empty Secret silently. | `templates/secret.yaml` now `fail`s the render when `externalSecret.enabled=false` and no data/existingSecret is given. |
| DEV-1 | **Low** | `deployments/seaweedfs-s3.json` shipped `admin/admin` dev creds under a prod-looking name. | Renamed to `seaweedfs-s3.dev.json` (compose references updated) so it cannot be mistaken for a prod artifact. |

## Recommended (not yet applied - require a product/ops decision)

### Token lifecycle - **Medium**
- Stateless JWTs can't be revoked before expiry. Keep the Keycloak access-token
  TTL short (≈5 min) and document it; for high-value ops consider token
  introspection.

### Edge / misc - **Medium/Low**
- `style-src 'unsafe-inline'` is required by antd's CSS-in-JS (CSS-injection /
  exfiltration risk). Track antd's static-extraction path to remove it.

## What's already strong

The codebase ships with notably careful security engineering:

- **JWT alg pinning** (`RS256/384/512` only) defeats `alg=none` and HMAC key
  confusion; `iss`+`aud`+`exp` all enforced, blocking cross-realm/client replay.
- **PKCE S256** with constant-time `state` comparison and short-lived transient
  cookies; all cookies `HttpOnly`+`Secure`+`SameSite=Lax`.
- **Credential isolation**: the user JWT/SigV4 headers are stripped before the
  request is re-signed to S3 with the service account (`rewrite.go`) - the user
  token never reaches storage.
- **Object-download hardening**: active content types (HTML/SVG/XML, `+xml`) are
  downgraded to inert `octet-stream` attachments with `default-src 'none';
  sandbox` CSP and `nosniff`; `Content-Disposition` filenames are sanitised
  against CR/LF/quote header injection.
- **Fail-closed ACL**: unparseable paths are treated as admin operations and
  blocked; a comprehensive bucket/object sub-resource blocklist.
- **Bounded outbound reads** (`io.LimitReader`, 1 MiB) on every token/discovery
  call; JWKS background refresh + startup retry.
- **Distroless non-root** runtime, read-only rootfs, dropped capabilities; ESO +
  Vault for secrets by default.

## How to re-run

```bash
make test-go            # unit/integration (the fixes above are covered)
govulncheck ./...       # recommended: dependency CVE scan (add to CI)
```
