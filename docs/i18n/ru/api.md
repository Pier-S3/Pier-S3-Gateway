# API-контракт - Pier S3 Gateway Web UI

> 🌍 Перевод. Источник истины - [английский docs/api.md](../../api.md).

## Аутентификация

Все запросы к `/api/v1/*` требуют Bearer-токен в заголовке `Authorization`
(или HttpOnly cookie).

## Эндпоинты

### GET /api/v1/auth/me
Возвращает информацию о текущем пользователе.
Ответ: `{"username": "alice", "groups": ["reports-ro", "dev-artifacts-rw"]}`

### POST /api/v1/auth/logout
Инвалидирует сессию через Keycloak `end_session_endpoint`.

### GET /api/v1/buckets
Возвращает список доступных бакетов с правами.

### GET /api/v1/buckets/{bucket}/objects?prefix=&delimiter=/&page_token=
Список объектов с пагинацией.

### GET /api/v1/buckets/{bucket}/objects/{key...}
Потоковая отдача объекта.

### GET /api/v1/buckets/{bucket}/meta/{key...}
Метаданные объекта (HeadObject).

> Примечание: метаданные используют **префиксный** маршрут
> `/meta/{key...}`, а не суффикс `/objects/{key...}/meta`, потому что mux в
> Go 1.22 не позволяет разместить литеральный сегмент после wildcard `{key...}`.

### PUT /api/v1/buckets/{bucket}/objects/{key...}
Потоковая загрузка объекта.

### DELETE /api/v1/buckets/{bucket}/objects/{key...}
Удаление объекта.

## Формат ошибок

`{"error": "error_code", "message": "Человекочитаемое описание"}`

Коды: `unauthorized`, `forbidden`, `write_not_allowed`,
`operation_not_permitted`, `not_found`, `bad_request`, `upstream_error`,
`backend_unavailable`, `internal_error`

## Усиление ответов (hardening)

Ответы с объектами и метаданными несут заголовки эшелонированной защиты, чтобы
недоверенный контент не мог выполниться в origin приложения:
- `X-Content-Type-Options: nosniff`
- `Content-Security-Policy: default-src 'none'; sandbox`
- Активные/опасные content-type (HTML/XHTML/SVG/XML, JavaScript, ...) отдаются
  как инертный `application/octet-stream` с `Content-Disposition: attachment`;
  безопасные типы отдаются inline с санитизированным именем файла.
