<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="../../assets/logo-horizontal-dark.svg">
    <img src="../../assets/logo-horizontal.svg" alt="Pier S3 Gateway" width="360">
  </picture>
</p>

<p align="center"><em>La jetée de votre stockage d'objets.</em></p>

# Pier S3 Gateway

**Pier S3 Gateway** est une passerelle proxy S3 dotée d'une interface web. Elle
fournit un accès autorisé à un stockage d'objets compatible S3 (SeaweedFS) via
Keycloak OIDC : vérification des JWT, ACL fondées sur les groupes/rôles de la
forme `<bucket>-<ro|rw|wo>` (plus les attributions joker `*` sur tous les
buckets) et proxy des opérations S3.

> 🌍 **Traductions :** [English](../../../README.md) (source de vérité). Les
> copies localisées se trouvent dans [`docs/i18n/`](../).

## Prérequis

- Go 1.22+
- Node.js 20+ / npm
- Docker et Docker Compose (pour le développement local)
- kubectl + un cluster Kubernetes (pour le déploiement)
- k6 (pour les tests de charge)

## Démarrage rapide

### 1. Installer les dépendances

```bash
# Dépendances Go
go mod download

# Dépendances du frontend
cd web && npm ci && cd ..
```

### 2. Compiler

```bash
# Build complet (frontend + backend)
make all

# Ou séparément
make frontend     # Vite build → internal/webui/static
make backend-dev  # Binaire Go (sans recompiler le frontend)
```

### 3. Lancer l'environnement de développement avec Docker Compose

```bash
# Démarrer SeaweedFS + Keycloak + la passerelle
make dev-up

# Logs
make dev-logs

# Arrêter
make dev-down
```

Une fois démarré :
- **Interface web** : http://localhost:8081
- **S3 Proxy** : http://localhost:8080
- **Keycloak Admin** : http://localhost:8180 (admin/admin)

### 4. Configurer Keycloak

1. Ouvrez http://localhost:8180 (admin / admin)
2. Créez un realm ou utilisez `master`
3. Créez le client `s3-proxy` (Client Protocol : openid-connect)
4. Créez un utilisateur et attribuez-lui des groupes/rôles de la forme
   `<bucket>-<policy>`, où `<policy>` vaut `ro` (lecture), `rw` (lecture +
   écriture + suppression) ou `wo` (écriture seule / dépôt) :
   - `reports-ro` - accès en lecture au bucket `reports`
   - `dev-artifacts-rw` - lecture, écriture et suppression dans `dev-artifacts`
   - `uploads-wo` - dépôt seul vers `uploads` (sans listage/téléchargement/suppression)
   - `*-ro` - accès en lecture à **tous** les buckets (joker ; p. ex. pour un auditeur)

> Runbook complet pour créer le realm / client / utilisateur (audience mapper,
> modèle rôles-vs-groupes et les pièges d'issuer/CORS/build Vite) :
> [docs/keycloak-setup.md](../../keycloak-setup.md)

## Tests

```bash
# Tous les tests
make test

# Tests Go uniquement
make test-go

# Tests avec rapport de couverture
make test-go-cover

# Modules individuels
make test-acl      # ACL resolver
make test-auth     # JWT/OIDC auth
make test-proxy    # S3 handler
make test-webui    # REST API
```

### Objectifs de couverture

| Module          | Couverture cible |
|-----------------|------------------|
| internal/acl    | ≥ 90%            |
| internal/auth   | ≥ 85%            |
| internal/proxy  | ≥ 80%            |
| internal/webui  | ≥ 80%            |

## Docker

```bash
# Construire l'image
make docker

# Vérifier la taille (cible : ≤ 30 MB)
make docker-size

# Lancer
make docker-run
```

## Kubernetes

### Helm (recommandé)

```bash
helm install pier-s3-gateway ./deployments/helm/pier-s3-gateway \
  --namespace pier-s3-gateway --create-namespace \
  --set image.repository=registry.example.com/pier-s3-gateway \
  --set image.tag=v1.0.0 \
  --set ingress.s3Host=s3.example.com \
  --set ingress.uiHost=s3-ui.example.com
```

Le chart paramètre tous les manifestes ci-dessous (replicas/HPA, hôtes ingress +
TLS, NetworkPolicy, PDB, ressources, security context) et prend en charge deux
sources de secrets : External Secrets Operator (par défaut) ou un Secret
simple/préexistant. Voir [`deployments/helm/pier-s3-gateway/README.md`](../../../deployments/helm/pier-s3-gateway/README.md).

### Manifestes bruts

```bash
# Appliquer tous les manifestes
make k8s-apply

# État du déploiement
make k8s-status

# Supprimer
make k8s-delete
```

### Manifestes

| Fichier | Description |
|---------|-------------|
| deployment.yaml | 2 réplicas, probes, limites de ressources, securityContext |
| service.yaml | ClusterIP : 8080 (S3) + 8081 (UI) |
| ingress.yaml | TLS via cert-manager, deux hôtes |
| secret.yaml | ExternalSecret (ESO) → Vault |
| networkpolicy.yaml | Ingress depuis Nginx, Egress vers Keycloak + SeaweedFS |
| hpa.yaml | 2-10 réplicas, target CPU 60% |
| pdb.yaml | minAvailable: 1 |

## Tests de charge

```bash
# k6 : 500 RPS, 5 minutes, GET/PUT mixtes
make load-test

# Ou avec des paramètres
k6 run -e BASE_URL=http://pier-s3-gateway.example.com -e AUTH_TOKEN=<jwt> tests/load/k6-script.js
```

Critères : latence p99 ≤ 5ms (surcoût d'autorisation), error_rate < 0.1%

## Architecture

```
┌───────────────────────────────────────────────────┐
│                   Ingress (TLS)                   │
│pier-s3-gateway.example.com  s3-ui.example.com     │
└───────────┬───────────────────────┬───────────────┘
            │ :8080                 │ :8081
┌───────────▼───────────┐ ┌────────▼───────────────┐
│    S3 Proxy Handler   │ │    Web UI (React SPA)  │
│  JWT → ACL → Proxy    │ │  REST API + Static     │
└───────────┬───────────┘ └────────┬───────────────┘
            │                      │
    ┌───────▼──────────────────────▼──────┐
    │         internal/auth (JWKS)        │
    │         internal/acl (groups)       │
    └───────┬──────────────────┬──────────┘
            │                  │
    ┌───────▼──────┐   ┌──────▼───────┐
    │  SeaweedFS   │   │   Keycloak   │
    │  (S3 API)    │   │   (OIDC)     │
    └──────────────┘   └──────────────┘
```

## Structure du projet

```
├── cmd/server/main.go          # Point d'entrée : deux serveurs HTTP + graceful shutdown
├── internal/
│   ├── config/config.go        # Charger la configuration depuis env
│   ├── auth/
│   │   ├── keycloak.go         # Vérification des JWT via JWKS
│   │   ├── claims.go           # Extraire username/groups des claims
│   │   └── oidc.go             # OIDC Authorization Code + PKCE
│   ├── acl/resolver.go         # Parsing des groupes, contrôle ACL, blocage des admin-ops
│   ├── proxy/
│   │   ├── s3client.go         # Client AWS SDK v2 (endpoint SeaweedFS)
│   │   ├── rewrite.go          # Réécriture des en-têtes
│   │   └── s3handler.go        # Handler HTTP : auth → ACL → proxy
│   └── webui/
│       ├── auth.go             # Auth middleware, /auth/me, /auth/logout
│       ├── api.go              # REST API (buckets, CRUD des objets)
│       ├── content_security.go # Durcissement des réponses (CSP, nosniff, neutralisation des types)
│       └── embed.go            # go:embed static, SPA fallback + CSP
├── web/                        # React 18 + TypeScript + Ant Design 5
│   └── src/
│       ├── auth/               # Client OIDC (oidc-client-ts)
│       ├── api/client.ts       # Axios + intercepteur Bearer
│       ├── store/              # Zustand (buckets, browser)
│       ├── theme/              # Presets de thème (light/dark/system + presets de code)
│       ├── i18n/               # Traductions de l'UI (en par défaut + ru/es/it/fr)
│       ├── components/         # BucketList, ObjectBrowser, FilePreview, ...
│       └── pages/              # Login, Buckets, Browser
├── deployments/
│   ├── Dockerfile              # Multi-stage : node → golang → distroless
│   └── k8s/                    # 7 manifestes (Deployment, Service, Ingress, ...)
├── docs/                       # Documentation (en anglais ; traductions dans docs/i18n/)
├── tests/load/k6-script.js     # k6 : test de charge de 500 RPS
├── docker-compose.dev.yml      # Dev : SeaweedFS + Keycloak + passerelle
└── Makefile                    # Toutes les commandes de build et de test
```

## Documentation

- [Référence de l'API](api.md)
- [Runbook de configuration de Keycloak](../../keycloak-setup.md)
- [Fournisseurs d'authentification - feuille de route d'intégration](../../auth-providers.md)
- Traductions : [`docs/i18n/`](../)
