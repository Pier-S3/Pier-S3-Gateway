<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="../../assets/logo-horizontal-dark.svg">
    <img src="../../assets/logo-horizontal.svg" alt="Pier S3 Gateway" width="360">
  </picture>
</p>

<p align="center"><em>The pier for your object storage.</em></p>

# Pier S3 Gateway

> 🌍 Перевод. Источник истины - [английский README](../../../README.md);
> перевод может отставать.

**Pier S3 Gateway** - это S3 proxy gateway с Web UI. Он предоставляет
авторизованный доступ к S3-совместимому хранилищу объектов (SeaweedFS) через
Keycloak OIDC: проверка JWT, ACL по группам/ролям вида `<bucket>-<ro|rw|wo>`
(плюс wildcard-гранты `*` на все бакеты) и проксирование S3-операций.

## Требования

- Go 1.22+
- Node.js 20+ / npm
- Docker и Docker Compose (для локального запуска)
- kubectl + кластер Kubernetes (для деплоя)
- k6 (для нагрузочного тестирования)

## Быстрый старт

### 1. Установка зависимостей

```bash
# Go-зависимости
go mod download

# Зависимости фронтенда
cd web && npm ci && cd ..
```

### 2. Сборка

```bash
# Полная сборка (frontend + backend)
make all

# Или по отдельности
make frontend     # Vite build → internal/webui/static
make backend-dev  # Go-бинарь (без пересборки фронтенда)
```

### 3. Запуск dev-окружения через Docker Compose

```bash
# Поднять SeaweedFS + Keycloak + gateway
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
4. Создайте пользователя и назначьте группы/роли вида `<bucket>-<policy>`,
   где `<policy>` - одно из `ro` (чтение), `rw` (чтение + запись + удаление)
   или `wo` (только запись / загрузка):
   - `reports-ro` - чтение бакета `reports`
   - `dev-artifacts-rw` - чтение, запись и удаление в `dev-artifacts`
   - `uploads-wo` - только загрузка в `uploads` (без листинга/скачивания/удаления)
   - `*-ro` - чтение **всех** бакетов (wildcard; например, для аудитора)

> Полный runbook по созданию realm / client / user (маппер audience, модель
> ролей-vs-групп и подводные камни issuer/CORS/Vite-сборки):
> [docs/keycloak-setup.md](../../keycloak-setup.md)

## Тестирование

```bash
# Все тесты
make test

# Только Go-тесты
make test-go

# Тесты с отчётом покрытия
make test-go-cover

# Отдельные модули
make test-acl      # ACL resolver
make test-auth     # JWT/OIDC auth
make test-proxy    # S3 handler
make test-webui    # REST API
```

### Целевое покрытие

| Модуль          | Целевое покрытие |
|-----------------|------------------|
| internal/acl    | ≥ 90%            |
| internal/auth   | ≥ 85%            |
| internal/proxy  | ≥ 80%            |
| internal/webui  | ≥ 80%            |

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

### Helm (рекомендуется)

```bash
helm install pier-s3-gateway ./deployments/helm/pier-s3-gateway \
  --namespace pier-s3-gateway --create-namespace \
  --set image.repository=registry.example.com/pier-s3-gateway \
  --set image.tag=v1.0.0 \
  --set ingress.s3Host=s3.example.com \
  --set ingress.uiHost=s3-ui.example.com
```

Чарт параметризует все манифесты ниже (replicas/HPA, ingress-хосты + TLS,
NetworkPolicy, PDB, resources, securityContext) и поддерживает два источника
секретов: External Secrets Operator (по умолчанию) или обычный/существующий
Secret. См. [`deployments/helm/pier-s3-gateway/README.md`](../../../deployments/helm/pier-s3-gateway/README.md).

### Сырые манифесты

```bash
# Применить все манифесты
make k8s-apply

# Статус выката
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
| hpa.yaml | 2-10 реплик, target CPU 60% |
| pdb.yaml | minAvailable: 1 |

## Нагрузочное тестирование

```bash
# k6: 500 RPS, 5 минут, смешанные GET/PUT
make load-test

# Или с параметрами
k6 run -e BASE_URL=http://pier-s3-gateway.example.com -e AUTH_TOKEN=<jwt> tests/load/k6-script.js
```

Критерии: p99 latency ≤ 5ms (overhead авторизации), error_rate < 0.1%

## Архитектура

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

## Структура проекта

```
├── cmd/server/main.go          # Entrypoint: два HTTP-сервера + graceful shutdown
├── internal/
│   ├── config/config.go        # Загрузка конфигурации из env
│   ├── auth/
│   │   ├── keycloak.go         # JWKS-верификация JWT
│   │   ├── claims.go           # Извлечение username/groups из claims
│   │   └── oidc.go             # OIDC Authorization Code + PKCE
│   ├── acl/resolver.go         # Парсинг групп, ACL-проверка, блок admin-операций
│   ├── proxy/
│   │   ├── s3client.go         # Клиент AWS SDK v2 (endpoint SeaweedFS)
│   │   ├── rewrite.go          # Переписывание заголовков
│   │   └── s3handler.go        # HTTP handler: auth → ACL → proxy
│   └── webui/
│       ├── auth.go             # Auth middleware, /auth/me, /auth/logout
│       ├── api.go              # REST API (buckets, objects CRUD)
│       ├── content_security.go # Усиление ответов (CSP, nosniff, нейтрализация типов)
│       └── embed.go            # go:embed static, SPA fallback + CSP
├── web/                        # React 18 + TypeScript + Ant Design 5
│   └── src/
│       ├── auth/               # OIDC-клиент (oidc-client-ts)
│       ├── api/client.ts       # Axios + Bearer-интерцептор
│       ├── store/              # Zustand (buckets, browser)
│       ├── theme/              # Темы (light/dark/system + code-пресеты)
│       ├── i18n/               # Переводы UI (en по умолчанию + ru/es/it/fr)
│       ├── components/         # BucketList, ObjectBrowser, FilePreview, ...
│       └── pages/              # Login, Buckets, Browser
├── deployments/
│   ├── Dockerfile              # Multi-stage: node → golang → distroless
│   └── k8s/                    # 7 манифестов (Deployment, Service, Ingress, ...)
├── docs/                       # Документация (английская; переводы в docs/i18n/)
├── tests/load/k6-script.js     # k6: нагрузочный тест 500 RPS
├── docker-compose.dev.yml      # Dev: SeaweedFS + Keycloak + gateway
└── Makefile                    # Все команды сборки и тестов
```

## Документация

- [API reference](../../api.md)
- [Runbook по настройке Keycloak](../../keycloak-setup.md)
- Переводы: [`docs/i18n/`](../)
