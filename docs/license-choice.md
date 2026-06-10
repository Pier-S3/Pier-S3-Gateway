# License recommendation

**Primary recommendation: Apache License 2.0.** A `LICENSE` (Apache-2.0) and
`NOTICE` file have been added on that basis - change them if you decide otherwise.

## Why Apache-2.0 for this project

Pier is a self-hostable **security/infrastructure gateway** that organizations
will deploy, modify, extend, and possibly bundle. That goal set points clearly to
a permissive license, and Apache-2.0 over bare MIT, for five reasons:

1. **Explicit patent grant (the decisive factor).** Apache §3 grants a patent
   license with a retaliation clause. For a *security* product, MIT's silence on
   patents is a real concern for enterprise legal review. Apache-2.0 gives you
   MIT-level permissiveness **plus** patent protection.
2. **Maximizes the adoption you want.** Self-host, modify, bundle are exactly the
   uses copyleft (AGPL) obstructs. Apache-2.0 lets corporate adopters and
   downstream products integrate Pier without copyleft anxiety.
3. **Ecosystem alignment.** Your dependency `aws-sdk-go-v2` is Apache-2.0, and the
   cloud-native world Pier lives in (Kubernetes, Dex, Keycloak, Ory, Casdoor) is
   overwhelmingly Apache-2.0. Zero compatibility friction; you signal "standard
   cloud-native infra."
4. **Avoids the MinIO trap.** MinIO relicensed to AGPL-3.0 and then stripped the
   management console from its community edition - widely read as a rug-pull.
   Clean permissive licensing is a competitive differentiator in exactly this
   category, and you can still run an **open-core** model (proprietary enterprise
   add-on) without relicensing the core or taking the AGPL reputation hit.
5. **Dependency licenses permit it.** `golang-jwt` (MIT), Ant Design (MIT), and
   `aws-sdk-go-v2` (Apache-2.0) are all permissive and compatible. There is no
   copyleft dependency forcing your hand.

## Runner-up: MPL-2.0

Choose **MPL-2.0** if you want a mild defensive copyleft: modifications to Pier's
*own source files* must be shared, while organizations may still link/bundle it
into proprietary systems. It keeps the patent grant, does **not** trigger the
network-use disclosure that frightens corporate legal, and is well-precedented
for security infra (Kanidm, Logto, OpenBao). Pick it if "prevent silent
privatization of the core" outweighs "absolute maximum adoption."

## What to avoid here

- **AGPL-3.0 / SSPL / BSL** - protect against SaaS free-riding but directly
  conflict with the bundle/modify/extend goals, and (post-MinIO) carry
  reputational baggage in this exact category. Only revisit AGPL if defending
  against managed-SaaS competitors becomes more important than adoption - and
  then commit to never gating community features.
- **Bare MIT** - fine and popular, but the missing patent grant is a real
  downgrade for a security product; Apache-2.0 dominates it at little cost.

## Housekeeping when you publish

- Keep `LICENSE` (Apache-2.0 full text) and `NOTICE` at the repo root.
- Add a short SPDX header (`// SPDX-License-Identifier: Apache-2.0`) to source
  files if you want machine-readable provenance.
- Track third-party dependency licenses (e.g. `go-licenses report ./...`) in your
  release process to keep the NOTICE current.
