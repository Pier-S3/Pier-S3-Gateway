# Contrato de API - Pier S3 Gateway Web UI

> 🌍 Traducción. La fuente de verdad es el [docs/api.md en inglés](../../api.md).

## Autenticación

Todas las peticiones a `/api/v1/*` requieren un token Bearer en la cabecera
`Authorization` (o una cookie HttpOnly).

## Endpoints

### GET /api/v1/auth/me
Devuelve información sobre el usuario actual.
Respuesta: `{"username": "alice", "groups": ["reports-ro", "dev-artifacts-rw"]}`

### POST /api/v1/auth/logout
Invalida la sesión a través del `end_session_endpoint` de Keycloak.

### GET /api/v1/buckets
Devuelve la lista de buckets accesibles con sus permisos.

### GET /api/v1/buckets/{bucket}/objects?prefix=&delimiter=/&page_token=
Lista los objetos con paginación.

### GET /api/v1/buckets/{bucket}/objects/{key...}
Entrega un objeto en streaming.

### GET /api/v1/buckets/{bucket}/meta/{key...}
Metadatos del objeto (HeadObject).

> Nota: los metadatos usan una ruta con **prefijo** `/meta/{key...}` en lugar de
> un sufijo `/objects/{key...}/meta`, porque el mux de Go 1.22 no permite colocar
> un segmento literal después de un comodín `{key...}`.

### PUT /api/v1/buckets/{bucket}/objects/{key...}
Sube un objeto en streaming.

### DELETE /api/v1/buckets/{bucket}/objects/{key...}
Elimina un objeto.

## Formato de errores

`{"error": "error_code", "message": "Descripción legible"}`

Códigos: `unauthorized`, `forbidden`, `write_not_allowed`,
`operation_not_permitted`, `not_found`, `bad_request`, `upstream_error`,
`backend_unavailable`, `internal_error`

## Endurecimiento de las respuestas (hardening)

Las respuestas de objetos y metadatos llevan cabeceras de defensa en profundidad
para que el contenido no confiable nunca pueda ejecutarse en el origin de la app:
- `X-Content-Type-Options: nosniff`
- `Content-Security-Policy: default-src 'none'; sandbox`
- Los content-type activos/peligrosos (HTML/XHTML/SVG/XML, JavaScript, ...) se
  entregan como un `application/octet-stream` inerte con
  `Content-Disposition: attachment`; los tipos seguros se entregan inline con un
  nombre de archivo saneado.
