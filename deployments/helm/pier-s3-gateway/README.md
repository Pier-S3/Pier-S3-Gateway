# Pier S3 Gateway - Helm chart

Deploys [Pier S3 Gateway](https://github.com/Pier-S3/Pier-S3-Gateway): an
authorizing S3 proxy with a web UI (Keycloak OIDC + group/role ACL) in front of
an S3-compatible store such as SeaweedFS.

## Install

```bash
helm install pier-s3-gateway ./deployments/helm/pier-s3-gateway \
  --namespace pier-s3-gateway --create-namespace \
  --set image.repository=registry.example.com/pier-s3-gateway \
  --set image.tag=v1.0.0 \
  --set ingress.s3Host=s3.example.com \
  --set ingress.uiHost=s3-ui.example.com
```

Upgrade / uninstall:

```bash
helm upgrade pier-s3-gateway ./deployments/helm/pier-s3-gateway -n pier-s3-gateway
helm uninstall pier-s3-gateway -n pier-s3-gateway
```

## Secrets

The gateway reads its S3 and Keycloak settings from env. Pick one source:

**External Secrets Operator (default).** Pulls values from a secret store
(e.g. Vault). No secret values live in the chart:

```yaml
externalSecret:
  enabled: true
  secretStore: { name: vault-backend, kind: ClusterSecretStore }
  remoteKey: secret/data/pier-s3-gateway
```

**Plain Secret (dev only).** Helm renders a `Secret` from `secret.data`:

```yaml
externalSecret: { enabled: false }
secret:
  create: true
  data:
    S3_ACCESS_KEY: "..."
    S3_SECRET_KEY: "..."
    S3_ENDPOINT: "http://seaweedfs:8333"
    S3_REGION: "us-east-1"
    KEYCLOAK_URL: "https://keycloak.example.com"
    KEYCLOAK_REALM: "master"
    KEYCLOAK_CLIENT_ID: "s3-proxy"
    KEYCLOAK_CLIENT_SECRET: ""        # empty for a public PKCE client
    KEYCLOAK_JWKS_URL: "https://keycloak.example.com/realms/master/protocol/openid-connect/certs"
```

**Pre-existing Secret.** `externalSecret.enabled=false`,
`secret.create=false`, `secret.existingSecret=my-secret`.

## Key values

| Key | Default | Description |
|-----|---------|-------------|
| `replicaCount` | `2` | Replicas (ignored when autoscaling is on). |
| `image.repository` / `image.tag` | `registry.example.com/pier-s3-gateway` / chart appVersion | Container image. |
| `service.s3Port` / `service.uiPort` | `8080` / `8081` | Container + service ports. |
| `config.logLevel` | `info` | `LOG_LEVEL` env. |
| `externalSecret.enabled` | `true` | Use ESO instead of a plain Secret. |
| `autoscaling.enabled` | `true` | HPA on CPU (`minReplicas`..`maxReplicas`). |
| `podDisruptionBudget.enabled` | `true` | PDB with `minAvailable`. |
| `ingress.enabled` | `true` | Two hosts: `s3Host` (→8080), `uiHost` (→8081). |
| `ingress.tls.enabled` / `ingress.tls.secretName` | `true` / `pier-s3-gateway-tls` | TLS via cert-manager. |
| `networkPolicy.enabled` | `true` | Ingress from the controller ns; egress DNS + Keycloak + SeaweedFS. |
| `resources` | 100m/128Mi .. 500m/256Mi | Requests/limits. |

See [`values.yaml`](values.yaml) for the full list, including the hardened
`podSecurityContext` / `securityContext` (non-root, read-only rootfs, dropped
caps) that mirror the raw manifests.

## Render locally

```bash
helm lint ./deployments/helm/pier-s3-gateway
helm template pier ./deployments/helm/pier-s3-gateway
```
