# API Contract - Pier S3 Gateway Web UI

> 🌍 Translations: [Русский](i18n/ru/api.md). This English version is the source
> of truth.

## Authentication

All requests to `/api/v1/*` require a Bearer token in the `Authorization`
header (or an HttpOnly cookie).

## Endpoints

### GET /api/v1/auth/me
Returns information about the current user.
Response: `{"username": "alice", "groups": ["reports-ro", "dev-artifacts-rw"]}`

### POST /api/v1/auth/logout
Invalidates the session via the Keycloak `end_session_endpoint`.

### GET /api/v1/buckets
Returns the list of accessible buckets with permissions.

### GET /api/v1/buckets/{bucket}/objects?prefix=&delimiter=/&page_token=
Lists objects with pagination.

### GET /api/v1/buckets/{bucket}/objects/{key...}
Streams an object.

### GET /api/v1/buckets/{bucket}/meta/{key...}
Object metadata (HeadObject).

> Note: metadata uses a `/meta/{key...}` prefix route rather than a
> `/objects/{key...}/meta` suffix, because Go 1.22's mux cannot place a literal
> segment after a `{key...}` wildcard.

### PUT /api/v1/buckets/{bucket}/objects/{key...}
Streams an object upload.

### DELETE /api/v1/buckets/{bucket}/objects/{key...}
Deletes an object.

## Error format

`{"error": "error_code", "message": "Human-readable description"}`

Codes: `unauthorized`, `forbidden`, `write_not_allowed`,
`operation_not_permitted`, `not_found`, `bad_request`, `upstream_error`,
`backend_unavailable`, `internal_error`

## Response hardening

Object and metadata responses carry defense-in-depth headers so untrusted
content can never execute in the app origin:
- `X-Content-Type-Options: nosniff`
- `Content-Security-Policy: default-src 'none'; sandbox`
- Active/dangerous content types (HTML/XHTML/SVG/XML, JavaScript, ...) are
  served as an inert `application/octet-stream` attachment; safe types are
  served inline with a sanitized `Content-Disposition` filename.
