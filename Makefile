.PHONY: all build test lint clean frontend backend docker run help

# Переменные
GO         := go
NPM        := npm
DOCKER     := docker
BINARY     := pier-s3
IMAGE_NAME := pier-s3-gateway
IMAGE_TAG  := latest

help: ## Показать справку
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

all: frontend backend ## Собрать всё (frontend + backend)

# ─── Frontend ──────────────────────────────────────────
frontend-deps: ## Установить зависимости фронтенда
	cd web && $(NPM) install

frontend: frontend-deps ## Собрать фронтенд (Vite → internal/webui/static)
	cd web && $(NPM) run build

frontend-dev: frontend-deps ## Запустить фронтенд в dev-режиме
	cd web && $(NPM) run dev

frontend-lint: ## Линтинг фронтенда
	cd web && $(NPM) run lint

frontend-test: ## Тесты фронтенда (Vitest)
	cd web && $(NPM) run test

# ─── Backend ───────────────────────────────────────────
backend-deps: ## Скачать Go-зависимости
	$(GO) mod download
	$(GO) mod tidy

backend: frontend ## Собрать Go-бинарник (включает frontend build)
	CGO_ENABLED=0 $(GO) build -ldflags="-s -w" -trimpath -o bin/$(BINARY) ./cmd/server

backend-dev: ## Собрать Go-бинарник без фронтенда (для разработки)
	$(GO) build -o bin/$(BINARY) ./cmd/server

# ─── Тесты ─────────────────────────────────────────────
test: test-go test-frontend ## Запустить все тесты

test-go: ## Go unit-тесты
	$(GO) test ./internal/... -v -count=1

test-go-cover: ## Go тесты с отчётом покрытия
	$(GO) test ./internal/... -v -count=1 -coverprofile=coverage.out
	$(GO) tool cover -func=coverage.out
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Отчёт покрытия: coverage.html"

test-frontend: ## Тесты фронтенда
	cd web && $(NPM) run test

test-acl: ## Тесты ACL resolver
	$(GO) test ./internal/acl/... -v -count=1

test-auth: ## Тесты auth
	$(GO) test ./internal/auth/... -v -count=1

test-proxy: ## Тесты S3 proxy handler
	$(GO) test ./internal/proxy/... -v -count=1

test-webui: ## Тесты Web UI API
	$(GO) test ./internal/webui/... -v -count=1

# ─── Линтинг ───────────────────────────────────────────
lint: lint-go lint-frontend ## Линтинг всего

lint-go: ## Go lint (go vet)
	$(GO) vet ./...

lint-frontend: ## Frontend lint (ESLint)
	cd web && $(NPM) run lint

# ─── Docker ────────────────────────────────────────────
docker: ## Собрать Docker-образ
	$(DOCKER) build -f deployments/Dockerfile -t $(IMAGE_NAME):$(IMAGE_TAG) .

docker-run: docker ## Собрать и запустить в Docker
	$(DOCKER) run --rm -p 8080:8080 -p 8081:8081 --env-file .env $(IMAGE_NAME):$(IMAGE_TAG)

docker-size: docker ## Показать размер Docker-образа
	@$(DOCKER) images $(IMAGE_NAME):$(IMAGE_TAG) --format "Размер образа: {{.Size}}"

# ─── Docker Compose (dev) ──────────────────────────────
dev-up: ## Запустить dev-окружение (docker-compose)
	$(DOCKER) compose -f docker-compose.dev.yml up -d

dev-down: ## Остановить dev-окружение
	$(DOCKER) compose -f docker-compose.dev.yml down

dev-logs: ## Логи dev-окружения
	$(DOCKER) compose -f docker-compose.dev.yml logs -f

# ─── Multi-provider e2e (Dex + LDAP) ───────────────────
dex-up: ## Запустить Dex/LDAP multi-provider окружение
	$(DOCKER) compose -f docker-compose.dex.yml up -d --build

dex-down: ## Остановить Dex/LDAP окружение
	$(DOCKER) compose -f docker-compose.dex.yml down -v

dex-logs: ## Логи Dex/LDAP окружения
	$(DOCKER) compose -f docker-compose.dex.yml logs -f

# ─── Kubernetes ────────────────────────────────────────
k8s-apply: ## Применить K8s манифесты
	kubectl apply -f deployments/k8s/

k8s-status: ## Статус деплоя
	kubectl rollout status deployment/pier-s3-gateway

k8s-delete: ## Удалить K8s ресурсы
	kubectl delete -f deployments/k8s/

# ─── Нагрузочное тестирование ──────────────────────────
load-test: ## Запустить k6 нагрузочный тест
	k6 run tests/load/k6-script.js

# ─── Утилиты ───────────────────────────────────────────
clean: ## Очистить артефакты сборки
	rm -rf bin/ coverage.out coverage.html
	rm -rf web/dist web/node_modules
	rm -rf internal/webui/static/*.js internal/webui/static/*.css

run: backend ## Собрать и запустить локально
	./bin/$(BINARY)

# ─── Helm ──────────────────────────────────────────────
HELM           := helm
HELM_RELEASE   := pier-s3-gateway
HELM_NAMESPACE := pier-s3-gateway
HELM_CHART     := deployments/helm/pier-s3-gateway

helm-lint: ## Линтинг Helm-чарта
	$(HELM) lint $(HELM_CHART)

helm-template: ## Рендер шаблонов чарта
	$(HELM) template $(HELM_RELEASE) $(HELM_CHART)

helm-install: ## Установить чарт в кластер
	$(HELM) install $(HELM_RELEASE) $(HELM_CHART) \
		--namespace $(HELM_NAMESPACE) --create-namespace

helm-upgrade: ## Обновить (или установить) релиз
	$(HELM) upgrade --install $(HELM_RELEASE) $(HELM_CHART) \
		--namespace $(HELM_NAMESPACE) --create-namespace

helm-uninstall: ## Удалить релиз
	$(HELM) uninstall $(HELM_RELEASE) --namespace $(HELM_NAMESPACE)
