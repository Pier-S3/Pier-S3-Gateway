# Supply-chain note: for reproducible, tamper-resistant builds, pin each base
# image by digest, e.g.
#   FROM node:20-alpine@sha256:<digest> AS frontend
# Resolve the current digest with:
#   docker buildx imagetools inspect node:20-alpine
# and keep them current with Renovate/Dependabot. Tags are used below so the
# build works out of the box; pin before shipping production images.

# Stage 1: Build frontend
FROM node:20-alpine AS frontend
WORKDIR /build/web
COPY web/package.json web/package-lock.json* ./
# npm ci installs exactly from the lockfile (reproducible) and fails if
# package.json and the lockfile are out of sync.
RUN npm ci
COPY web/ ./
# Vite bakes VITE_* at build time; pass the browser-facing Keycloak realm URL
# and client id so the embedded SPA points at the right IdP. See
# docs/keycloak-setup.md (G2). Defaults keep prior behaviour when unset.
ARG VITE_KEYCLOAK_URL
ARG VITE_KEYCLOAK_CLIENT_ID
ENV VITE_KEYCLOAK_URL=${VITE_KEYCLOAK_URL} \
    VITE_KEYCLOAK_CLIENT_ID=${VITE_KEYCLOAK_CLIENT_ID}
RUN npm run build

# Stage 2: Build Go binary
FROM golang:1.22-alpine AS backend
WORKDIR /build
COPY go.mod go.sum* ./
# Do NOT mask download failures (a missing/altered module must fail the build,
# not silently fall through to a stale cache).
RUN go mod download
COPY . .
COPY --from=frontend /build/internal/webui/static ./internal/webui/static
# -mod=readonly: build strictly from the committed go.mod/go.sum; never mutate
# them or pull undeclared modules during the image build.
RUN CGO_ENABLED=0 GOOS=linux go build -mod=readonly -ldflags="-s -w" -trimpath -o /app ./cmd/server

# Stage 3: Runtime
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=backend /app /app
USER 65532:65532
EXPOSE 8080 8081 8082
ENTRYPOINT ["/app"]
