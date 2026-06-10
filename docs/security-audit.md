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

## Recommended (not yet applied - require a product/ops decision)

### Discovery issuer cross-check - **Low** (largely mitigated)
With SSRF-1, the discovery URL is now fetched over TLS, closing the practical
MITM path. As a further belt-and-suspenders step, when both `OIDC_ISSUER` and
`OIDC_DISCOVERY_URL` are set you may assert the discovered issuer equals the
configured one. (`Config.ResolveOIDC`)

### Cookie scope & token lifecycle - **Medium**
- Set `access_token`/`refresh_token` cookie `Path=/api/` (not `/`) so they aren't
  sent with static-asset requests. **Caveat:** the matching `clearCookie` calls
  must use the same path or logout won't delete them.
- Stateless JWTs can't be revoked before expiry. Keep the Keycloak access-token
  TTL short (≈5 min) and document it; for high-value ops consider token
  introspection.
- Token-endpoint error bodies are interpolated into error strings that get logged
  at `Error` level. Log only the status + parsed `error` code. (`internal/auth/oidc.go`)

### Edge / misc - **Medium/Low**
- `/_health` `/_ready` were reachable through the public ingress - the provided
  NGINX config now returns 404 for them; mirror that in the k8s Ingress.
- `style-src 'unsafe-inline'` is required by antd's CSS-in-JS (CSS-injection /
  exfiltration risk). Track antd's static-extraction path to remove it.
- Set an explicit `redirect_uri` and allowlist it at the IdP (defence against any
  future open-redirect regression).
- Helm: `fail` the render when `externalSecret.enabled=false` **and** no secret
  data/existingSecret is provided, instead of creating an empty Secret.
- `deployments/seaweedfs-s3.json` ships `admin/admin` dev creds - mark clearly as
  dev-only and keep out of any prod path.

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
