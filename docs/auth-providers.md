# Authentication providers - integration roadmap

Pier's authorization core is **standard OIDC + JWKS**: it validates a Bearer
JWT's signature (RS256/384/512), `exp`, `iss` and `aud`, then maps claims to a
bucket ACL. See `internal/auth/{keycloak,claims,oidc}.go`.

Because the core is generic, most OSS identity providers do **not** need a code
integration - they need (a) the right claim mapping and (b) documentation. This
roadmap is ordered so each phase unlocks the next.

> Status legend: `[ ]` planned, `[~]` partial, `[x]` done. Effort: Low / Medium / High.

---

## What works today

Pier authenticates against **Keycloak** (OIDC, Authorization Code + PKCE). The
verifier already reads two group/role shapes:

- `groups` (flat array of strings)
- `realm_access.roles` (Keycloak-nested)

Identity comes from `preferred_username`, falling back to `sub`. ACL groups must
be shaped `<bucket>-<ro|rw|wo>` (plus `*` wildcard).

The current coupling to Keycloak is small and limited to:

| Keycloak-ism | Where | Generic equivalent |
|--------------|-------|--------------------|
| issuer built as `<url>/realms/<realm>` | `oidc.go` `IssuerURL` | a full configurable issuer URL |
| `realm_access.roles` claim | `claims.go` | a configurable groups/roles claim |
| env names `KEYCLOAK_*` | `config.go` | provider-neutral `OIDC_*` names |

---

## Phase 1 - Provider-agnostic OIDC (foundation)  `[ ]`

Goal: any compliant OIDC IdP works by configuration alone, no code changes.

Work:
1. **Configurable claim mapping** (new env, keep `KEYCLOAK_*` as aliases):
   - `OIDC_ISSUER` - full issuer string to match `iss` (replaces realm assembly).
   - `OIDC_JWKS_URL` - JWKS endpoint.
   - `OIDC_AUDIENCE` - expected `aud` (often the client id).
   - `OIDC_GROUPS_CLAIM` - dotted path to the groups/roles claim
     (`groups`, `roles`, `realm_access.roles`, `resource_access.<client>.roles`,
     `urn:zitadel:iam:org:project:roles`, `cognito:groups`, ...).
   - `OIDC_USERNAME_CLAIM` - default `preferred_username`.
2. **Dotted-path + map-shaped claim extraction** in `claims.go` (some IdPs put
   roles in an object keyed by role name rather than a string array).
3. **OIDC discovery (optional):** read `/.well-known/openid-configuration` to
   auto-fill `issuer`/`jwks_uri` from one `OIDC_DISCOVERY_URL`.
4. Tests for each claim shape; docs + example env per provider.

Outcome: turns Pier from a "Keycloak gateway" into an "OIDC gateway." Everything
below becomes config + a short guide.

---

## Phase 2 - First-class OSS OIDC providers  `[ ]`

Each is a guide + a verified example config (and a compose profile for e2e),
once Phase 1 lands.

| Provider | Notes | Effort |
|----------|-------|--------|
| **Dex** | OIDC federator (LDAP, GitHub, SAML upstreams). `groups` scope out of the box. Great "one connector, many backends" story. | Low |
| **Authentik** | Full OIDC; add a group-membership scope mapper emitting `groups`. Maps cleanly to the existing extractor. | Low |
| **Zitadel** | OIDC; roles in `urn:zitadel:iam:org:project:roles` (an object) - needs the map-shaped extraction from Phase 1. | Medium |
| **Ory Hydra** | OAuth2/OIDC server; groups injected at the consent step. Pairs with Kratos for identities. | Medium |
| **Authelia** | OIDC provider + forward-auth; `groups` claim available. | Low |
| **Gitea / Forgejo, GitLab** | OIDC providers - useful for dev-team-scoped deployments; map org/team to bucket groups. | Medium |

---

## Phase 3 - Enterprise / non-OIDC  `[ ]`

| Mechanism | Approach | Effort |
|-----------|----------|--------|
| **LDAP / Active Directory** | Prefer **federation via Dex or Keycloak** (LDAP -> OIDC) rather than a native LDAP backend - keeps Pier OIDC-only. Native LDAP bind is a fallback if direct integration is required. | Medium-High |
| **SAML 2.0** | No native SAML in Pier; broker through Dex/Keycloak/Authentik (SAML upstream -> OIDC downstream). | Medium (via broker) |
| **OAuth2 token introspection (RFC 7662)** | Support opaque (non-JWT) access tokens by calling the IdP's introspection endpoint, with short-TTL caching. Enables IdPs that don't issue JWTs. | Medium |
| **mTLS / client certificates** | Service-to-service auth for the S3 API path (map cert SAN/CN to an identity + groups). | Medium |

---

## Cross-cutting (any phase)

- **JWKS rotation/caching** is already handled (background refresh + startup
  retry).
- **Multiple audiences / multiple issuers** - support a list, for gateways
  fronting more than one client or realm.
- **Group-claim size limits** - some IdPs page or truncate large group sets;
  document and, if needed, support the token's distributed-claims pointer.
- **Conformance tests** - a small matrix in CI that boots each provider
  (Dex/Authentik/Zitadel) via compose and runs the same auth e2e.

---

## Suggested order

1. **Phase 1** (config-driven claim mapping + discovery) - the multiplier.
2. **Dex + Authentik** guides (lowest effort, broadest reach; Dex also covers
   LDAP/SAML/GitHub upstreams).
3. **Zitadel** (exercises map-shaped roles) and **token introspection**
   (unlocks opaque-token IdPs).
4. Enterprise LDAP/SAML via broker, then mTLS for service clients.

Nothing here changes the ACL model (`<bucket>-<ro|rw|wo>` + `*`) or the
always-blocked admin operations - only how identities and groups arrive.
