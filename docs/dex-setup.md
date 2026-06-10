# Multi-provider end-to-end: Pier + Dex + LDAP

This profile authenticates Pier with **Dex** (federating an **OpenLDAP**
directory) instead of Keycloak - proving the gateway is a genuine OIDC gateway,
not a Keycloak-specific one. The **only** difference from the Keycloak setup is
`OIDC_*` configuration; no gateway code changes.

It implements the roadmap's flagship story
([auth-providers.md](./auth-providers.md), [iam-providers.md](./iam-providers.md)):
**Dex brokers LDAP/AD/SAML/GitHub upstreams into a flat `groups` claim** that maps
directly onto Pier's `<bucket>-<ro|rw|wo>` (plus `*`) ACL.

## Run it

```bash
make dex-up          # docker compose -f docker-compose.dex.yml up -d --build
make dex-logs        # follow logs
make dex-down        # tear down (and drop volumes)
```

Services:

| Service | URL | Notes |
|---------|-----|-------|
| Pier Web UI | http://localhost:8081 | log in here |
| Pier S3 proxy | http://localhost:8080 | Bearer-token S3 API |
| Dex (OIDC) | http://localhost:5556/dex | issuer + discovery |
| OpenLDAP | localhost:1389 | seeded directory |

## Log in

Open **http://localhost:8081** → **Sign in** → at the Dex screen choose **LDAP**
and authenticate as:

- **username:** `alice`
- **password:** `password`

Alice's LDAP group memberships arrive in the token's `groups` claim and grant:

| Group (LDAP cn) | Effect in Pier |
|-----------------|----------------|
| `reports-ro` | read the `reports` bucket |
| `uploads-wo` | upload-only to `uploads` |
| `*-ro` | read-only across **every** bucket (auditor wildcard) |

Create those buckets in SeaweedFS to see them appear, e.g. with the AWS CLI
against `http://localhost:8333` (creds `admin`/`admin`).

## How the wiring maps

| Concern | Keycloak setup | This Dex setup |
|---------|----------------|----------------|
| Issuer (`iss`) | `<KEYCLOAK_URL>/realms/<realm>` | `OIDC_ISSUER=http://localhost:5556/dex` |
| JWKS | `KEYCLOAK_JWKS_URL` | `OIDC_JWKS_URL=http://dex:5556/dex/keys` |
| Groups claim | `realm_access.roles` (default) | `OIDC_GROUPS_CLAIM=groups` |
| Username claim | `preferred_username` (default) | `OIDC_USERNAME_CLAIM=email` |
| SPA scopes | `openid profile` | `VITE_OIDC_SCOPE="openid profile email groups"` |

The browser-vs-cluster issuer split is identical to Keycloak: tokens carry the
**browser-facing** issuer (`localhost:5556`), while the gateway fetches JWKS over
the **in-cluster** service name (`dex:5556`). `OIDC_ALLOW_INSECURE=true` is set
because everything here is plaintext http on the compose network - **never** in
production.

## Status / caveats

These manifests are **validated for YAML + compose schema** but were **not run
end-to-end in the environment that generated them** (no Docker daemon was
available). Before relying on them, verify in your environment and note:

- **Image tags** (`dexidp/dex:v2.39.1`, `bitnami/openldap:2.6`,
  `chrislusf/seaweedfs:3.69`) - bump to current releases as needed.
- **LDAP seed**: `bitnami/openldap` loads `/ldifs/*.ldif` on first boot only;
  `make dex-down` drops the volume so a fresh `dex-up` re-seeds.
- If groups don't appear in `/api/v1/auth/me`, confirm the `groups` scope is
  requested (it is, via `VITE_OIDC_SCOPE`) and that Dex's `groupSearch` matches
  your directory layout.

## Swapping in other providers

Because the integration is configuration-only, pointing Pier at **Authentik**,
**Authelia**, **Zitadel**, **GitLab**, etc. is the same exercise: set
`OIDC_ISSUER`/`OIDC_JWKS_URL` (or `OIDC_DISCOVERY_URL`), the `OIDC_GROUPS_CLAIM`,
and the SPA scope. See [iam-providers.md](./iam-providers.md) for per-provider
claim shapes.
