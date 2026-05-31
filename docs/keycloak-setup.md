# Keycloak setup: realm, client, user

Step-by-step rollout guide for wiring Pier S3 Gateway to Keycloak. Every
requirement below is derived from what the gateway actually validates - see
`internal/auth` and `cmd/server/main.go`.

## What the gateway validates (read this first)

On every request the proxy verifies the Bearer **access token**
(`internal/auth/keycloak.go` + `claims.go`):

| Check | Required value | Source in code |
|-------|----------------|----------------|
| Signature | RS256 / RS384 / RS512, key from JWKS | `VerifyToken` |
| `exp` | must be present and valid | `jwt.WithExpirationRequired()` |
| `iss` | exactly `${KEYCLOAK_URL}/realms/${KEYCLOAK_REALM}` | `IssuerURL`, bound in `main.go` |
| `aud` | must contain `${KEYCLOAK_CLIENT_ID}` (e.g. `s3-proxy`) | `expectedAudience` |
| identity | `preferred_username`, fallback `sub` | `ExtractUser` |
| authorization | `groups[]` **or** `realm_access.roles[]`, each `<bucket>-<ro\|rw\|wo>` | `ExtractGroups`, `acl.parseGroup` |

ACL rule format (`internal/acl/resolver.go`): the **last** hyphen splits the
bucket from the policy. Policies:

| Policy | GET/HEAD/OPTIONS | PUT/POST | DELETE | Use |
|--------|:---:|:---:|:---:|-----|
| `ro`   | ✓ | - | - | read-only |
| `rw`   | ✓ | ✓ | ✓ | full access (write implies delete) |
| `wo`   | - | ✓ | - | write-only / upload (no read or delete) |

The bucket part may be the wildcard `*`, which applies the policy to **every**
bucket. A user's grants are the union of all their matching groups.

- `reports-ro`        -> read bucket `reports`
- `dev-artifacts-rw`  -> read+write+delete bucket `dev-artifacts`
- `uploads-wo`        -> upload-only to bucket `uploads`
- `*-ro`              -> read **every** bucket (e.g. an auditor/reader role)
- `*-rw`              -> full access to every bucket

Bucket/object administration (create/delete bucket, policy, ACL, versioning,
lifecycle, CORS, encryption, ...) is **always blocked** regardless of policy
(`IsAdminOperation`).

A leading `/` (Keycloak group paths) is stripped automatically.

---

## 1. Realm

- **Dev (docker-compose):** the compose uses realm `master` - nothing to create.
- **Prod / clean setup:** create a dedicated realm `s3` (Manage realms -> Create
  realm -> name `s3`). Then set `KEYCLOAK_REALM=s3` and the JWKS/issuer envs to
  that realm.

## 2. Client `s3-proxy`

**General settings**
- Client type: `OpenID Connect`
- Client ID: `s3-proxy` (must equal `KEYCLOAK_CLIENT_ID`)

**Capability config**
- Client authentication: **Off** (public client; auth is Authorization Code +
  PKCE, no secret needed - see `internal/auth/oidc.go`)
- Authorization: Off
- Authentication flow: **Standard flow** on. (Direct access grants optional, only
  for `curl`/scripts via password grant. Implicit: off.)

**Login settings** (host UI runs on `:8081`)
- Valid redirect URIs: `http://localhost:8081/callback`
  (prod: `https://s3-ui.example.com/callback`)
- Valid post logout redirect URIs: `http://localhost:8081`
  (prod: `https://s3-ui.example.com`)
- Web origins: `http://localhost:8081` (CORS - the SPA calls Keycloak's token
  endpoint cross-origin; without this the browser blocks login)

The SPA's `redirect_uri` is always `<origin>/callback` and
`post_logout_redirect_uri` is `<origin>` (see `web/src/auth/AuthProvider.tsx`).

### 2a. Audience mapper (REQUIRED - common gotcha)

By default a public client's access token does **not** list itself in `aud`, so
the gateway's audience check fails with `token verification failed`. Add it:

Clients -> `s3-proxy` -> Client scopes -> `s3-proxy-dedicated` -> Add mapper ->
By configuration -> **Audience**:
- Included client audience: `s3-proxy`
- Add to access token: **On**

## 3. Authorization model - pick ONE

The SPA requests `scope: 'openid profile'` only (`AuthProvider.tsx`), so the
claim you use must travel in a **default** scope, not an optional one.

### Option A - Realm roles (least config, recommended for quick start)
Works out of the box: realm roles ride in `realm_access.roles` via the default
`roles` scope, which the gateway reads.

1. Realm roles -> Create role -> `reports-ro` (repeat for each `<bucket>-<ro|rw|wo>`)
2. Assign in step 4 via the user's Role mapping.

### Option B - Groups (matches the README naming)
Groups are **not** in the token by default - add a mapper:

1. Groups -> Create group -> `reports-ro`, `dev-artifacts-rw`, ...
2. Clients -> `s3-proxy` -> Client scopes -> `s3-proxy-dedicated` -> Add mapper
   -> **Group Membership**:
   - Token Claim Name: `groups`
   - Full group path: **Off** (emit `reports-ro`, not `/reports-ro`)
   - Add to access token: **On**
3. Assign in step 4 via the user's Groups tab.

## 4. User

1. Users -> Add user -> Username (`alice`), Email, etc. -> Create
2. Credentials -> Set password -> Temporary: **Off** (dev)
3. Grant access:
   - Option A: Role mapping -> Assign role -> `reports-ro`, `dev-artifacts-rw`
   - Option B: Groups -> Join group -> `reports-ro`, ...

Verify the token (Direct access grants must be on for this):
```bash
curl -s -d "client_id=s3-proxy" -d "username=alice" -d "password=PASS" \
  -d "grant_type=password" \
  http://localhost:8180/realms/master/protocol/openid-connect/token | jq -r .access_token \
  | cut -d. -f2 | base64 -d 2>/dev/null | jq '{iss,aud,preferred_username,groups,realm_access}'
```
`aud` must contain `s3-proxy`; `groups` or `realm_access.roles` must contain your
`<bucket>-<ro|rw|wo>` entries.

---

## Gotchas specific to this project

### G1. Issuer / hostname must match the browser's view of Keycloak
The browser logs in via `http://localhost:8180`, so the token `iss` is
`http://localhost:8180/realms/master`. The proxy only *string-compares* `iss`
(it fetches keys from the separate `KEYCLOAK_JWKS_URL`), so its `KEYCLOAK_URL`
must use the **same host the browser sees**, while JWKS stays internal. The dev
compose is already configured this way (`docker-compose.dev.yml`, service
`pier-s3-gateway`):
```yaml
KEYCLOAK_URL: http://localhost:8180          # issuer match with browser tokens
KEYCLOAK_JWKS_URL: http://keycloak:8180/realms/master/protocol/openid-connect/certs  # fetched in-cluster
```
If you ever point `KEYCLOAK_URL` at the in-cluster name (`http://keycloak:8180`)
instead, browser tokens will be rejected (`iss` mismatch). Prod: serve Keycloak
on one canonical URL (ingress) used by both browser and proxy, and pin
`KC_HOSTNAME` so `iss` is deterministic.

### G2. Frontend env is baked at BUILD time
`VITE_KEYCLOAK_URL` / `VITE_KEYCLOAK_CLIENT_ID` are read at Vite **build** time
(`AuthProvider.tsx`) - not at runtime. `deployments/Dockerfile` accepts them as
build args (`ARG`/`ENV` before `npm run build`), and `docker-compose.dev.yml`
already passes them for the dev stack:
```yaml
# docker-compose.dev.yml, service pier-s3-gateway:
build:
  args:
    VITE_KEYCLOAK_URL: http://localhost:8180/realms/master
    VITE_KEYCLOAK_CLIENT_ID: s3-proxy
```
For your own image build, pass them explicitly, otherwise the SPA falls back to
the `keycloak.example.com` default and will NOT point at your Keycloak:
```bash
docker build -f deployments/Dockerfile \
  --build-arg VITE_KEYCLOAK_URL=https://keycloak.example.com/realms/<realm> \
  --build-arg VITE_KEYCLOAK_CLIENT_ID=s3-proxy \
  -t pier-s3-gateway:latest .
```
`VITE_KEYCLOAK_URL` is the realm URL the **browser** hits.

### G3. Keycloak startup race
The gateway retries the initial JWKS fetch (~60s) so a still-starting Keycloak no
longer crashes it. If you see repeated `JWKS init failed, retrying` it is benign
while Keycloak boots; it proceeds once `/certs` is reachable.

---

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `failed to initialize JWKS verifier ... connection refused` | Keycloak not up yet | wait (auto-retries) / G3 |
| `token verification failed` + audience | `aud` lacks `s3-proxy` | add Audience mapper (2a) |
| `token verification failed` + issuer | `iss` host mismatch | G1 |
| 403 on a bucket the user should access | role/group missing or wrong shape | must be exactly `<bucket>-<ro|rw|wo>`; check token claim (step 4) |
| Login redirect loop / CORS error in console | redirect URI or Web origins wrong | step 2 Login settings |
| SPA redirects to `keycloak.example.com` | frontend built without `VITE_KEYCLOAK_URL` | G2 |
