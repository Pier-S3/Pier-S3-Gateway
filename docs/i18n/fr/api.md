# Contrat d'API - Pier S3 Gateway Web UI

> 🌍 Traduction. La source de vérité est le [docs/api.md en anglais](../../api.md).

## Authentification

Toutes les requêtes vers `/api/v1/*` nécessitent un token Bearer dans l'en-tête
`Authorization` (ou un cookie HttpOnly).

## Endpoints

### GET /api/v1/auth/me
Renvoie les informations sur l'utilisateur courant.
Réponse : `{"username": "alice", "groups": ["reports-ro", "dev-artifacts-rw"]}`

### POST /api/v1/auth/logout
Invalide la session via l'`end_session_endpoint` de Keycloak.

### GET /api/v1/buckets
Renvoie la liste des buckets accessibles avec leurs permissions.

### GET /api/v1/buckets/{bucket}/objects?prefix=&delimiter=/&page_token=
Liste les objets avec pagination.

### GET /api/v1/buckets/{bucket}/objects/{key...}
Diffuse un objet en streaming.

### GET /api/v1/buckets/{bucket}/meta/{key...}
Métadonnées de l'objet (HeadObject).

> Remarque : les métadonnées utilisent une route avec **préfixe** `/meta/{key...}`
> plutôt qu'un suffixe `/objects/{key...}/meta`, car le mux de Go 1.22 ne permet
> pas de placer un segment littéral après un joker `{key...}`.

### PUT /api/v1/buckets/{bucket}/objects/{key...}
Téléverse un objet en streaming.

### DELETE /api/v1/buckets/{bucket}/objects/{key...}
Supprime un objet.

## Format des erreurs

`{"error": "error_code", "message": "Description lisible"}`

Codes : `unauthorized`, `forbidden`, `write_not_allowed`,
`operation_not_permitted`, `not_found`, `bad_request`, `upstream_error`,
`backend_unavailable`, `internal_error`

## Durcissement des réponses (hardening)

Les réponses d'objets et de métadonnées portent des en-têtes de défense en
profondeur afin que le contenu non fiable ne puisse jamais s'exécuter dans
l'origin de l'application :
- `X-Content-Type-Options: nosniff`
- `Content-Security-Policy: default-src 'none'; sandbox`
- Les content-type actifs/dangereux (HTML/XHTML/SVG/XML, JavaScript, ...) sont
  servis comme un `application/octet-stream` inerte avec
  `Content-Disposition: attachment` ; les types sûrs sont servis inline avec un
  nom de fichier assaini.
