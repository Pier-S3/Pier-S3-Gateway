# API Контракт - Pier S3 Gateway Web UI

> 🌍 Перевод. Источник истины - [английский docs/api.md](../../api.md).

## Аутентификация
Все запросы к `/api/v1/*` требуют Bearer-токен в заголовке `Authorization` или HttpOnly cookie.

## Эндпоинты

### GET /api/v1/auth/me
Возвращает информацию о текущем пользователе.
Response: `{"username": "alice", "groups": ["reports-ro", "dev-artifacts-rw"]}`

### POST /api/v1/auth/logout
Инвалидирует сессию через Keycloak end_session_endpoint.

### GET /api/v1/buckets
Возвращает список доступных бакетов с правами.

### GET /api/v1/buckets/{bucket}/objects?prefix=&delimiter=/&page_token=
Список объектов с пагинацией.

### GET /api/v1/buckets/{bucket}/objects/{key+}
Потоковая отдача объекта.

### GET /api/v1/buckets/{bucket}/objects/{key+}/meta
Метаданные объекта (HeadObject).

### PUT /api/v1/buckets/{bucket}/objects/{key+}
Потоковая загрузка объекта.

### DELETE /api/v1/buckets/{bucket}/objects/{key+}
Удаление объекта.

## Формат ошибок
`{"error": "error_code", "message": "Human-readable description"}`

Коды: unauthorized, forbidden, write_not_allowed, operation_not_permitted, not_found, internal_error
