# NGINX hardening for Pier S3 Gateway

A ready-to-adapt config lives at
[`deployments/nginx/pier-s3-gateway.conf`](../deployments/nginx/pier-s3-gateway.conf).
This document explains the header-control model and the security choices, so you
can audit or port them to another edge (Ingress, Envoy, HAProxy).

Pier sits behind a reverse proxy that terminates TLS and forwards to two ports:
the **S3 proxy** (`:8080`) and the **Web UI + REST API** (`:8081`). The proxy is
part of the trust boundary - several of the gateway's own defences assume the
edge sanitises headers.

## Header control - the core of this config

### 1. Strip and re-set every trust-bearing header

The gateway's per-IP rate limiter keys on the **right-most** `X-Forwarded-For`
hop precisely because a remote client can put anything in the **left-most**
position. NGINX must therefore guarantee that the value it appends is the only
one that can be trusted:

```nginx
proxy_set_header X-Forwarded-For   $remote_addr;   # overwrite, do not append
proxy_set_header X-Forwarded-Proto $scheme;
proxy_set_header X-Forwarded-Host  $host;
proxy_set_header X-Real-IP         $remote_addr;
proxy_set_header Forwarded         "";             # drop RFC 7239 form entirely
```

Using `$remote_addr` (not `$proxy_add_x_forwarded_for`) **overwrites** any
client-supplied chain with the real peer. If you have *N* trusted proxies in
front of NGINX, switch to `real_ip` (`set_real_ip_from <trusted-cidr>;
real_ip_header X-Forwarded-For; real_ip_recursive on;`) so `$remote_addr` is the
genuine client, and extend the gateway to skip *N* right-most hops.

> Never pass the client's `X-Forwarded-*` through untouched. That is the single
> most common way a "behind a proxy" rate limiter or audit log gets spoofed.

### 2. Forward the end-user Authorization header

Counter-intuitively, the user's `Authorization: Bearer <jwt>` **must** reach the
gateway - the gateway verifies the JWT (signature/iss/aud/exp), maps claims to
the bucket ACL, and only then re-signs the request to the S3 backend with its
own service-account SigV4 credentials. The user's JWT never reaches storage, but
it must reach Pier:

```nginx
proxy_set_header Authorization $http_authorization;
```

The gateway strips the user's SigV4 and `Authorization` headers itself before
forwarding to S3 (`internal/proxy/rewrite.go`), so no credential leaks downstream.

### 3. Close hop-by-hop and smuggling vectors

```nginx
proxy_http_version 1.1;
proxy_set_header   Connection "";   # let nginx manage keepalive; drop client's
proxy_set_header   Proxy "";        # neutralise the legacy httpoxy header
```

`server_tokens off;` hides the version. `large_client_header_buffers` is bounded.
nginx already drops `Connection`-listed and standard hop-by-hop headers; the
explicit `Proxy ""` closes the [httpoxy](https://httpoxy.org) class.

### 4. Do not double-set the app CSP

The SPA serves its **own** Content-Security-Policy (`internal/webui/embed.go`),
which must allow the Keycloak origin for the OIDC token call and silent renew. A
second, stricter CSP at the edge would break login. This config sets
`X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, and HSTS, but
deliberately leaves CSP to the app. (The S3 proxy responses carry their own
sandboxed CSP for object downloads.)

## Streaming, not buffering

Both object paths can be multi-gigabyte. `proxy_request_buffering off` and
`proxy_buffering off` keep nginx from spooling whole objects to disk/memory, and
the read/send timeouts are raised to 600s. This matches the gateway, which runs
with `WriteTimeout: 0` for the same reason and relies on the edge timeouts to
bound slow-transfer abuse.

## Hide internal endpoints

`/_health` and `/_ready` are for in-cluster probes only. The config returns 404
for them at the public edge so the data plane surface isn't enumerable.

## Rate limiting (defence in depth)

`pier_ui_auth` (20 r/m) throttles the unauthenticated OIDC endpoints - the
brute-force surface - mirroring the gateway's own limiter. `pier_s3_api`
(50 r/s) and `limit_conn` bound the data plane. Tune to your traffic.

## Checklist

- [ ] Real hostnames + valid certs; HSTS only after full HTTPS rollout.
- [ ] `X-Forwarded-For` overwritten, not appended (or `real_ip` configured).
- [ ] `Authorization` forwarded; `Forwarded`/`Proxy` dropped.
- [ ] `client_max_body_size` set to your real max object size.
- [ ] `/_health` + `/_ready` return 404 publicly.
- [ ] Upstreams use keepalive and a stable address (avoid startup-only DNS).
- [ ] Edge does **not** set Content-Security-Policy on the UI host.
