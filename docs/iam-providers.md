# Open-source IAM providers - integration catalog

This catalogs the OSS identity providers Pier can authenticate against and how
much work each takes. For the phased _integration plan_ (config-driven claim
mapping, discovery, broker strategy) see
[auth-providers.md](./auth-providers.md). This page is the **provider reference**.

## The only question that matters

Pier validates a JWT against a JWKS endpoint and reads one claim to derive groups
→ bucket ACL. So integrating any provider reduces to: **which claim carries the
groups, and what shape is it?**

- **Flat array of strings** (`["reports-ro", "uploads-wo"]`) → near-zero effort;
  set `OIDC_GROUPS_CLAIM` and go.
- **Object keyed by role/name** (Keycloak `realm_access.roles`, Zitadel project
  roles) → needs the map-shaped extraction Pier already supports
  (`auth.ClaimMapper` accepts array, object-keyed, and string shapes).
- **Custom claim name** (Kanidm) → just point `OIDC_GROUPS_CLAIM` at it.

Group names must still resolve to the `<bucket>-<ro|rw|wo>` (plus `*`) convention.

## Provider matrix

| Provider | License | OIDC | Groups claim (shape) | Effort | Notes |
|----------|---------|------|----------------------|--------|-------|
| **Keycloak** | Apache-2.0 | Native | `realm_access.roles` / `resource_access.<c>.roles` (object) or `groups` via mapper | Done | Reference target; add a client mapper to also emit a flat `groups[]`. |
| **Dex** | Apache-2.0 | Federator | `groups` (array) | **Low** | Best **broker**: LDAP/AD, SAML, GitHub upstreams → one flat `groups[]`. Top first integration. |
| **Authentik** | core MIT (enterprise add-ons commercial) | Native | `groups` (array, configurable mappers) | **Low** | Popular self-host; flexible property mappings. |
| **Authelia** | Apache-2.0 | Native (OP) | `groups` (array) | **Low** | Lightweight; common alongside reverse proxies. |
| **Pocket ID** | BSD-2-Clause | Native (passkey-first) | `groups` (array) | **Low** | Tiny, passkey-only, "just works" - great demo target. |
| **GitLab** | core MIT (EE proprietary) | Native (OP) | `groups` / `groups_direct` (array of paths) | **Low-Med** | Instant SSO for orgs already on GitLab. |
| **Gitea / Forgejo** | Gitea MIT / **Forgejo GPL-3.0** | Native (OP) | `groups` (array, version-dependent) | **Med** | Git host doubles as IdP for small teams. |
| **Casdoor** | Apache-2.0 | Native | `roles`/`groups` (array) | **Low-Med** | Broad protocol support; Apache-2.0 friendly. |
| **Zitadel** | **AGPL-3.0** (since v3; ≤v2 Apache-2.0) | Native | `urn:zitadel:iam:org:project:roles` (object) | **Med** | Exercises object-keyed extraction. Note the license flip. |
| **Logto** | MPL-2.0 | Native (OAuth 2.1) | `roles` (array); org roles via scopes | **Med** | RBAC-first, developer-friendly. |
| **Kanidm** | MPL-2.0 | Native | user-specified claim name (group values) | **Med** | Security-focused (Rust); don't hardcode `groups`. |
| **Ory Hydra** (+ Kratos) | Apache-2.0 | Hydra = certified OP | you define claims at the consent app | **Med** | Clean OP, but bring-your-own claim logic. |
| **SuperTokens** | core Apache-2.0 | OAuth/OIDC (newer) | `roles` via session claims | **Med** | Session-mgmt-first; more config to emit standard claims. |
| **Glewlwyd** | GPL-3.0 (libs LGPL-2.1) | Native OP | scope/claim configurable per plugin | **Med** | Niche, lightweight C implementation. |
| **FusionAuth** (Community) | **proprietary** (FusionAuth CLA - not OSI) | Native | `roles` (array) | **Low** | Free Community tier, strong OIDC - but **not** open source. Flag for users who require OSS. |

> Licenses verified June 2026 against upstream where possible; reconfirm the
> exact SPDX string from each project's `LICENSE` before relying on it. Notable
> nuances: **Forgejo** is GPL-3.0 (Gitea is MIT); **Zitadel** is AGPL-3.0 from
> v3.0; **FusionAuth Community** is proprietary, not OSS.

## Recommended order

1. **Keycloak** (reference) - also ship the object-keyed role adapter.
2. **Dex** - the universal broker; flat `groups[]`; unlocks LDAP/AD/SAML/GitHub.
3. **Authentik**, **Authelia**, **Pocket ID** - lightweight, flat `groups[]`,
   huge self-host/homelab reach and excellent demos.
4. **GitLab** - flat group-path array; instant value for existing GitLab orgs.
5. **Zitadel** + **Kanidm** - exercise object-keyed / custom-named claims.

## Non-OIDC: LDAP / Active Directory, SAML

Don't build native LDAP/SAML into Pier. **Broker through Dex or Keycloak**
(LDAP/AD/SAML upstream → OIDC downstream with a flat `groups[]`). This keeps
Pier's auth surface to one protocol (OIDC + JWKS) while reaching the entire
enterprise-directory world via one well-understood hop.

## Design implication

Make the group source a configurable **claim-path + shape mode**
(`array-of-strings | object-keyed | space-delimited-string`). That single feature
covers essentially every provider in the matrix - which is exactly the direction
`auth.ClaimMapper` already takes.
