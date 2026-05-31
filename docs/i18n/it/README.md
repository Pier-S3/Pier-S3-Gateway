<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="../../assets/logo-horizontal-dark.svg">
    <img src="../../assets/logo-horizontal.svg" alt="Pier S3 Gateway" width="360">
  </picture>
</p>

<p align="center"><em>Il molo per il tuo object storage.</em></p>

# Pier S3 Gateway

**Pier S3 Gateway** è un gateway proxy S3 con interfaccia web. Fornisce accesso
autorizzato a un object store compatibile con S3 (SeaweedFS) tramite Keycloak
OIDC: verifica dei JWT, ACL basate su gruppi/ruoli nella forma
`<bucket>-<ro|rw|wo>` (più concessioni wildcard `*` su tutti i bucket) e proxy
delle operazioni S3.

> 🌍 **Traduzioni:** [English](../../../README.md) (fonte di verità). Le copie
> localizzate si trovano in [`docs/i18n/`](../).

## Requisiti

- Go 1.22+
- Node.js 20+ / npm
- Docker e Docker Compose (per lo sviluppo locale)
- kubectl + un cluster Kubernetes (per il deployment)
- k6 (per i test di carico)

## Avvio rapido

### 1. Installare le dipendenze

```bash
# Dipendenze Go
go mod download

# Dipendenze del frontend
cd web && npm ci && cd ..
```

### 2. Compilare

```bash
# Build completa (frontend + backend)
make all

# Oppure separatamente
make frontend     # Vite build → internal/webui/static
make backend-dev  # Binario Go (senza ricompilare il frontend)
```

### 3. Avviare l'ambiente di sviluppo con Docker Compose

```bash
# Avviare SeaweedFS + Keycloak + il gateway
make dev-up

# Log
make dev-logs

# Arrestare
make dev-down
```

Una volta avviato:
- **Interfaccia web**: http://localhost:8081
- **S3 Proxy**: http://localhost:8080
- **Keycloak Admin**: http://localhost:8180 (admin/admin)

### 4. Configurare Keycloak

1. Apri http://localhost:8180 (admin / admin)
2. Crea un realm o usa `master`
3. Crea il client `s3-proxy` (Client Protocol: openid-connect)
4. Crea un utente e assegnagli gruppi/ruoli nella forma `<bucket>-<policy>`,
   dove `<policy>` è uno tra `ro` (lettura), `rw` (lettura + scrittura +
   cancellazione) o `wo` (solo scrittura / upload):
   - `reports-ro` - accesso in lettura al bucket `reports`
   - `dev-artifacts-rw` - lettura, scrittura e cancellazione in `dev-artifacts`
   - `uploads-wo` - solo upload su `uploads` (senza listing/download/cancellazione)
   - `*-ro` - accesso in lettura a **tutti** i bucket (wildcard; es. per un auditor)

> Runbook completo per creare realm / client / utente (audience mapper, modello
> ruoli-vs-gruppi e le insidie di issuer/CORS/build Vite):
> [docs/keycloak-setup.md](../../keycloak-setup.md)

## Test

```bash
# Tutti i test
make test

# Solo test Go
make test-go

# Test con report di copertura
make test-go-cover

# Moduli singoli
make test-acl      # ACL resolver
make test-auth     # JWT/OIDC auth
make test-proxy    # S3 handler
make test-webui    # REST API
```

### Obiettivi di copertura

| Modulo          | Copertura obiettivo |
|-----------------|---------------------|
| internal/acl    | ≥ 90%               |
| internal/auth   | ≥ 85%               |
| internal/proxy  | ≥ 80%               |
| internal/webui  | ≥ 80%               |

## Docker

```bash
# Costruire l'immagine
make docker

# Verificare la dimensione (obiettivo: ≤ 30 MB)
make docker-size

# Eseguire
make docker-run
```

## Kubernetes

### Helm (consigliato)

```bash
helm install pier-s3-gateway ./deployments/helm/pier-s3-gateway \
  --namespace pier-s3-gateway --create-namespace \
  --set image.repository=registry.example.com/pier-s3-gateway \
  --set image.tag=v1.0.0 \
  --set ingress.s3Host=s3.example.com \
  --set ingress.uiHost=s3-ui.example.com
```

Il chart parametrizza tutti i manifest seguenti (replicas/HPA, host ingress +
TLS, NetworkPolicy, PDB, risorse, security context) e supporta due sorgenti di
segreti: External Secrets Operator (predefinito) o un Secret semplice/esistente.
Vedi [`deployments/helm/pier-s3-gateway/README.md`](../../../deployments/helm/pier-s3-gateway/README.md).

### Manifest grezzi

```bash
# Applicare tutti i manifest
make k8s-apply

# Stato del deployment
make k8s-status

# Eliminare
make k8s-delete
```

### Manifest

| File | Descrizione |
|------|-------------|
| deployment.yaml | 2 repliche, probe, limiti di risorse, securityContext |
| service.yaml | ClusterIP: 8080 (S3) + 8081 (UI) |
| ingress.yaml | TLS tramite cert-manager, due host |
| secret.yaml | ExternalSecret (ESO) → Vault |
| networkpolicy.yaml | Ingress da Nginx, Egress verso Keycloak + SeaweedFS |
| hpa.yaml | 2-10 repliche, target CPU 60% |
| pdb.yaml | minAvailable: 1 |

## Test di carico

```bash
# k6: 500 RPS, 5 minuti, GET/PUT misti
make load-test

# Oppure con parametri
k6 run -e BASE_URL=http://pier-s3-gateway.example.com -e AUTH_TOKEN=<jwt> tests/load/k6-script.js
```

Criteri: latenza p99 ≤ 5ms (overhead di autorizzazione), error_rate < 0.1%

## Architettura

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

## Struttura del progetto

```
├── cmd/server/main.go          # Entrypoint: due server HTTP + graceful shutdown
├── internal/
│   ├── config/config.go        # Caricamento della configurazione da env
│   ├── auth/
│   │   ├── keycloak.go         # Verifica dei JWT tramite JWKS
│   │   ├── claims.go           # Estrazione di username/groups dai claim
│   │   └── oidc.go             # OIDC Authorization Code + PKCE
│   ├── acl/resolver.go         # Parsing dei gruppi, controllo ACL, blocco delle admin-op
│   ├── proxy/
│   │   ├── s3client.go         # Client AWS SDK v2 (endpoint SeaweedFS)
│   │   ├── rewrite.go          # Riscrittura delle header
│   │   └── s3handler.go        # Handler HTTP: auth → ACL → proxy
│   └── webui/
│       ├── auth.go             # Auth middleware, /auth/me, /auth/logout
│       ├── api.go              # REST API (bucket, CRUD degli oggetti)
│       ├── content_security.go # Hardening delle risposte (CSP, nosniff, neutralizzazione dei tipi)
│       └── embed.go            # go:embed static, SPA fallback + CSP
├── web/                        # React 18 + TypeScript + Ant Design 5
│   └── src/
│       ├── auth/               # Client OIDC (oidc-client-ts)
│       ├── api/client.ts       # Axios + interceptor Bearer
│       ├── store/              # Zustand (bucket, browser)
│       ├── theme/              # Preset dei temi (light/dark/system + preset di codice)
│       ├── i18n/               # Traduzioni della UI (en predefinito + ru/es/it/fr)
│       ├── components/         # BucketList, ObjectBrowser, FilePreview, ...
│       └── pages/              # Login, Buckets, Browser
├── deployments/
│   ├── Dockerfile              # Multi-stage: node → golang → distroless
│   └── k8s/                    # 7 manifest (Deployment, Service, Ingress, ...)
├── docs/                       # Documentazione (in inglese; traduzioni in docs/i18n/)
├── tests/load/k6-script.js     # k6: test di carico da 500 RPS
├── docker-compose.dev.yml      # Dev: SeaweedFS + Keycloak + gateway
└── Makefile                    # Tutti i comandi di build e test
```

## Documentazione

- [Riferimento dell'API](api.md)
- [Runbook di configurazione di Keycloak](../../keycloak-setup.md)
- [Provider di autenticazione - roadmap di integrazione](../../auth-providers.md)
- Traduzioni: [`docs/i18n/`](../)
