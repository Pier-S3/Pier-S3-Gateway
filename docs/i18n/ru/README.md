# Pier S3 Gateway

> 🌍 Перевод. Источник истины - [английский README](../../../README.md).

> The pier for your object storage.

**Pier S3 Gateway** - S3 proxy gateway с Web UI. Обеспечивает авторизованный доступ к S3-совместимому хранилищу (SeaweedFS) через Keycloak OIDC: проверка JWT, ACL по группам/ролям вида `<bucket>-<ro|rw>` и проксирование S3-операций.

## Требования

- Go 1.22+
- Node.js 20+ / npm
- Docker & Docker Compose (для локального запуска)
- kubectl + кластер K8s (для деплоя)
- k6 (для нагрузочного тестирования)

## Быстрый старт

### 1. Установка зависимостей

```bash
# Go-зависимости
go mod download

# Frontend-зависимости
cd web && npm ci && cd ..
```

### 2. Сборка

```bash
# Полная сборка (frontend + backend)
make all

# Или по отдельности
make frontend     # Vite build → internal/webui/static
make backend-dev  # Go binary (без пересборки фронтенда)
```

### 3. Запуск dev-окружения через Docker Compose

```bash
# Поднять SeaweedFS + Keycloak + S3 Proxy
make dev-up

# Логи
make dev-logs

# Остановить
make dev-down
```

После запуска:
- **Web UI**: http://localhost:8081
- **S3 Proxy**: http://localhost:8080
- **Keycloak Admin**: http://localhost:8180 (admin/admin)

### 4. Настройка Keycloak

1. Откройте http://localhost:8180 (admin / admin)
2. Создайте realm или используйте `master`
3. Создайте client `s3-proxy` (Client Protocol: openid-connect)
4. Создайте пользователя и назначьте ему группы вида `<bucket>-<policy>`:
   - `reports-ro` — чтение бакета `reports`
   - `dev-artifacts-rw` — чтение и запись в `dev-artifacts`

> Полный runbook по заведению realm / client / user (с маппером audience,
> моделью ролей-vs-групп и разбором подводных камней issuer/CORS/VITE-сборки):
> [docs/keycloak-setup.md](../../keycloak-setup.md)

## Тестирование

```bash
# Все тесты
make test

# Только Go-тесты
make test-go

# Тесты с покрытием
make test-go-cover

# Отдельные модули
make test-acl      # ACL resolver
make test-auth     # JWT/OIDC auth
make test-proxy    # S3 handler
make test-webui    # REST API
```

### Ожидаемое покрытие

| Модуль          | Целевое покрытие |
|-----------------|-----------------|
| internal/acl    | ≥ 90%           |
| internal/auth   | ≥ 85%           |
| internal/proxy  | ≥ 80%           |
| internal/webui  | ≥ 80%           |

## Docker

```bash
# Собрать образ
make docker

# Проверить размер (цель: ≤ 30 MB)
make docker-size

# Запустить
make docker-run
```

## Kubernetes

```bash
# Применить все манифесты
make k8s-apply

# Проверить статус
make k8s-status

# Удалить
make k8s-delete
```

### Манифесты

| Файл | Описание |
|------|----------|
| deployment.yaml | 2 реплики, probes, resource limits, securityContext |
| service.yaml | ClusterIP: 8080 (S3) + 8081 (UI) |
| ingress.yaml | TLS через cert-manager, два хоста |
| secret.yaml | ExternalSecret (ESO) → Vault |
| networkpolicy.yaml | Ingress от Nginx, Egress к Keycloak + SeaweedFS |
| hpa.yaml | 2–10 реплик, target CPU 60% |
| pdb.yaml | minAvailable: 1 |

## Нагрузочное тестирование

```bash
# k6: 500 RPS, 5 минут, смешанные GET/PUT
make load-test

# Или с параметрами
k6 run -e BASE_URL=http://pier.example.com -e AUTH_TOKEN=<jwt> tests/load/k6-script.js
```

Критерии: p99 latency ≤ 5ms (overhead авторизации), error_rate < 0.1%

## Архитектура

```
┌──────────────────────────────────────────────────┐
│                   Ingress (TLS)                   │
│          pier.example.com  s3-ui.example.com  │
└───────────┬───────────────────────┬──────────────┘
            │ :8080                 │ :8081
┌───────────▼───────────┐ ┌────────▼───────────────┐
│    S3 Proxy Handler   │ │    Web UI (React SPA)   │
│  JWT → ACL → Proxy    │ │  REST API + Static      │
└───────────┬───────────┘ └────────┬───────────────┘
            │                      │
    ┌───────▼──────────────────────▼──────┐
    │         internal/auth (JWKS)         │
    │         internal/acl (groups)        │
    └───────┬──────────────────┬──────────┘
            │                  │
    ┌───────▼──────┐   ┌──────▼───────┐
    │  SeaweedFS   │   │   Keycloak   │
    │  (S3 API)    │   │   (OIDC)     │
    └──────────────┘   └──────────────┘
```

## Структура проекта

```
├── cmd/server/main.go          # Entrypoint: два HTTP сервера + graceful shutdown
├── internal/
│   ├── config/config.go        # Загрузка конфигурации из env
│   ├── auth/
│   │   ├── keycloak.go         # JWKS верификация JWT
│   │   ├── claims.go           # Извлечение username/groups из claims
│   │   └── oidc.go             # OIDC Authorization Code + PKCE
│   ├── acl/resolver.go         # Парсинг групп, ACL check, admin-op blocking
│   ├── proxy/
│   │   ├── s3client.go         # AWS SDK v2 клиент (SeaweedFS endpoint)
│   │   ├── rewrite.go          # Перепись заголовков
│   │   └── s3handler.go        # HTTP handler: auth → ACL → proxy
│   └── webui/
│       ├── auth.go             # Auth middleware, /auth/me, /auth/logout
│       ├── api.go              # REST API (buckets, objects CRUD)
│       └── embed.go            # go:embed static, SPA fallback
├── web/                        # React 18 + TypeScript + Ant Design 5
│   └── src/
│       ├── auth/               # OIDC client (oidc-client-ts)
│       ├── api/client.ts       # Axios + Bearer interceptor
│       ├── store/              # Zustand (buckets, browser)
│       ├── components/         # BucketList, ObjectBrowser, UploadZone, ...
│       └── pages/              # Login, Buckets, Browser
├── deployments/
│   ├── Dockerfile              # Multi-stage: node → golang → distroless
│   └── k8s/                    # 7 манифестов (Deployment, Service, Ingress, ...)
├── tests/load/k6-script.js     # k6: 500 RPS нагрузочный тест
├── docker-compose.dev.yml      # Dev: SeaweedFS + Keycloak + S3 Proxy
└── Makefile                    # Все команды сборки и тестирования
```
