# Contratto API - Pier S3 Gateway Web UI

> 🌍 Traduzione. La fonte di verità è il [docs/api.md in inglese](../../api.md).

## Autenticazione

Tutte le richieste a `/api/v1/*` richiedono un token Bearer nell'header
`Authorization` (o un cookie HttpOnly).

## Endpoint

### GET /api/v1/auth/me
Restituisce informazioni sull'utente corrente.
Risposta: `{"username": "alice", "groups": ["reports-ro", "dev-artifacts-rw"]}`

### POST /api/v1/auth/logout
Invalida la sessione tramite l'`end_session_endpoint` di Keycloak.

### GET /api/v1/buckets
Restituisce l'elenco dei bucket accessibili con i relativi permessi.

### GET /api/v1/buckets/{bucket}/objects?prefix=&delimiter=/&page_token=
Elenca gli oggetti con paginazione.

### GET /api/v1/buckets/{bucket}/objects/{key...}
Restituisce un oggetto in streaming.

### GET /api/v1/buckets/{bucket}/meta/{key...}
Metadati dell'oggetto (HeadObject).

> Nota: i metadati usano una rotta con **prefisso** `/meta/{key...}` invece di un
> suffisso `/objects/{key...}/meta`, perché il mux di Go 1.22 non consente di
> collocare un segmento letterale dopo un wildcard `{key...}`.

### PUT /api/v1/buckets/{bucket}/objects/{key...}
Carica un oggetto in streaming.

### DELETE /api/v1/buckets/{bucket}/objects/{key...}
Elimina un oggetto.

## Formato degli errori

`{"error": "error_code", "message": "Descrizione leggibile"}`

Codici: `unauthorized`, `forbidden`, `write_not_allowed`,
`operation_not_permitted`, `not_found`, `bad_request`, `upstream_error`,
`backend_unavailable`, `internal_error`

## Hardening delle risposte

Le risposte di oggetti e metadati portano header di difesa in profondità affinché
il contenuto non attendibile non possa mai essere eseguito nell'origin dell'app:
- `X-Content-Type-Options: nosniff`
- `Content-Security-Policy: default-src 'none'; sandbox`
- I content-type attivi/pericolosi (HTML/XHTML/SVG/XML, JavaScript, ...) vengono
  serviti come un `application/octet-stream` inerte con
  `Content-Disposition: attachment`; i tipi sicuri vengono serviti inline con un
  nome file sanificato.
