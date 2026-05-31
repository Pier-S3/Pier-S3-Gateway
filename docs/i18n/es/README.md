<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="../../assets/logo-horizontal-dark.svg">
    <img src="../../assets/logo-horizontal.svg" alt="Pier S3 Gateway" width="360">
  </picture>
</p>

<p align="center"><em>El muelle para tu almacenamiento de objetos.</em></p>

# Pier S3 Gateway

**Pier S3 Gateway** es una pasarela proxy S3 con interfaz web. Proporciona
acceso autorizado a un almacén de objetos compatible con S3 (SeaweedFS) a través
de Keycloak OIDC: verificación de JWT, ACL basada en grupos/roles con la forma
`<bucket>-<ro|rw|wo>` (más concesiones comodín `*` sobre todos los buckets) y
proxy de las operaciones S3.

> 🌍 **Traducciones:** [English](../../../README.md) (fuente de verdad). Las
> copias localizadas viven en [`docs/i18n/`](../).

## Requisitos

- Go 1.22+
- Node.js 20+ / npm
- Docker y Docker Compose (para desarrollo local)
- kubectl + un clúster de Kubernetes (para el despliegue)
- k6 (para pruebas de carga)

## Inicio rápido

### 1. Instalar dependencias

```bash
# Dependencias de Go
go mod download

# Dependencias del frontend
cd web && npm ci && cd ..
```

### 2. Compilar

```bash
# Compilación completa (frontend + backend)
make all

# O por separado
make frontend     # Vite build → internal/webui/static
make backend-dev  # Binario Go (sin recompilar el frontend)
```

### 3. Ejecutar el entorno de desarrollo con Docker Compose

```bash
# Levantar SeaweedFS + Keycloak + la pasarela
make dev-up

# Logs
make dev-logs

# Detener
make dev-down
```

Una vez levantado:
- **Interfaz web**: http://localhost:8081
- **S3 Proxy**: http://localhost:8080
- **Keycloak Admin**: http://localhost:8180 (admin/admin)

### 4. Configurar Keycloak

1. Abre http://localhost:8180 (admin / admin)
2. Crea un realm o usa `master`
3. Crea el cliente `s3-proxy` (Client Protocol: openid-connect)
4. Crea un usuario y asígnale grupos/roles con la forma `<bucket>-<policy>`,
   donde `<policy>` es uno de `ro` (lectura), `rw` (lectura + escritura +
   borrado) o `wo` (solo escritura / subida):
   - `reports-ro` - acceso de lectura al bucket `reports`
   - `dev-artifacts-rw` - lectura, escritura y borrado en `dev-artifacts`
   - `uploads-wo` - solo subida a `uploads` (sin listar/descargar/borrar)
   - `*-ro` - acceso de lectura a **todos** los buckets (comodín; p. ej. para un auditor)

> Runbook completo para crear el realm / cliente / usuario (audience mapper,
> modelo de roles-vs-grupos y los problemas de issuer/CORS/compilación Vite):
> [docs/keycloak-setup.md](../../keycloak-setup.md)

## Pruebas

```bash
# Todas las pruebas
make test

# Solo pruebas Go
make test-go

# Pruebas con informe de cobertura
make test-go-cover

# Módulos individuales
make test-acl      # ACL resolver
make test-auth     # JWT/OIDC auth
make test-proxy    # S3 handler
make test-webui    # REST API
```

### Objetivos de cobertura

| Módulo          | Cobertura objetivo |
|-----------------|--------------------|
| internal/acl    | ≥ 90%              |
| internal/auth   | ≥ 85%              |
| internal/proxy  | ≥ 80%              |
| internal/webui  | ≥ 80%              |

## Docker

```bash
# Construir la imagen
make docker

# Comprobar el tamaño (objetivo: ≤ 30 MB)
make docker-size

# Ejecutar
make docker-run
```

## Kubernetes

### Helm (recomendado)

```bash
helm install pier-s3-gateway ./deployments/helm/pier-s3-gateway \
  --namespace pier-s3-gateway --create-namespace \
  --set image.repository=registry.example.com/pier-s3-gateway \
  --set image.tag=v1.0.0 \
  --set ingress.s3Host=s3.example.com \
  --set ingress.uiHost=s3-ui.example.com
```

El chart parametriza todos los manifiestos siguientes (replicas/HPA, hosts de
ingress + TLS, NetworkPolicy, PDB, recursos, security context) y admite dos
fuentes de secretos: External Secrets Operator (por defecto) o un Secret
plano/preexistente. Consulta [`deployments/helm/pier-s3-gateway/README.md`](../../../deployments/helm/pier-s3-gateway/README.md).

### Manifiestos sin procesar

```bash
# Aplicar todos los manifiestos
make k8s-apply

# Estado del despliegue
make k8s-status

# Eliminar
make k8s-delete
```

### Manifiestos

| Archivo | Descripción |
|---------|-------------|
| deployment.yaml | 2 réplicas, probes, límites de recursos, securityContext |
| service.yaml | ClusterIP: 8080 (S3) + 8081 (UI) |
| ingress.yaml | TLS mediante cert-manager, dos hosts |
| secret.yaml | ExternalSecret (ESO) → Vault |
| networkpolicy.yaml | Ingress desde Nginx, Egress a Keycloak + SeaweedFS |
| hpa.yaml | 2-10 réplicas, target CPU 60% |
| pdb.yaml | minAvailable: 1 |

## Pruebas de carga

```bash
# k6: 500 RPS, 5 minutos, GET/PUT mixtos
make load-test

# O con parámetros
k6 run -e BASE_URL=http://pier-s3-gateway.example.com -e AUTH_TOKEN=<jwt> tests/load/k6-script.js
```

Criterios: latencia p99 ≤ 5ms (sobrecarga de autorización), error_rate < 0.1%

## Arquitectura

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

## Estructura del proyecto

```
├── cmd/server/main.go          # Punto de entrada: dos servidores HTTP + graceful shutdown
├── internal/
│   ├── config/config.go        # Cargar la configuración desde env
│   ├── auth/
│   │   ├── keycloak.go         # Verificación de JWT mediante JWKS
│   │   ├── claims.go           # Extraer username/groups de los claims
│   │   └── oidc.go             # OIDC Authorization Code + PKCE
│   ├── acl/resolver.go         # Parseo de grupos, comprobación de ACL, bloqueo de admin-ops
│   ├── proxy/
│   │   ├── s3client.go         # Cliente AWS SDK v2 (endpoint SeaweedFS)
│   │   ├── rewrite.go          # Reescritura de cabeceras
│   │   └── s3handler.go        # Handler HTTP: auth → ACL → proxy
│   └── webui/
│       ├── auth.go             # Auth middleware, /auth/me, /auth/logout
│       ├── api.go              # REST API (buckets, CRUD de objetos)
│       ├── content_security.go # Endurecimiento de respuestas (CSP, nosniff, neutralización de tipos)
│       └── embed.go            # go:embed static, SPA fallback + CSP
├── web/                        # React 18 + TypeScript + Ant Design 5
│   └── src/
│       ├── auth/               # Cliente OIDC (oidc-client-ts)
│       ├── api/client.ts       # Axios + interceptor Bearer
│       ├── store/              # Zustand (buckets, browser)
│       ├── theme/              # Presets de tema (light/dark/system + presets de código)
│       ├── i18n/               # Traducciones de UI (en por defecto + ru/es/it/fr)
│       ├── components/         # BucketList, ObjectBrowser, FilePreview, ...
│       └── pages/              # Login, Buckets, Browser
├── deployments/
│   ├── Dockerfile              # Multi-stage: node → golang → distroless
│   └── k8s/                    # 7 manifiestos (Deployment, Service, Ingress, ...)
├── docs/                       # Documentación (en inglés; traducciones en docs/i18n/)
├── tests/load/k6-script.js     # k6: prueba de carga de 500 RPS
├── docker-compose.dev.yml      # Dev: SeaweedFS + Keycloak + pasarela
└── Makefile                    # Todos los comandos de compilación y pruebas
```

## Documentación

- [Referencia de la API](api.md)
- [Runbook de configuración de Keycloak](../../keycloak-setup.md)
- [Proveedores de autenticación - hoja de ruta de integración](../../auth-providers.md)
- Traducciones: [`docs/i18n/`](../)
