# Stage 1: Build frontend
FROM node:20-alpine AS frontend
WORKDIR /build/web
COPY web/package.json web/package-lock.json* ./
RUN npm install --prefer-offline
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
RUN go mod download || true
COPY . .
RUN go mod tidy
COPY --from=frontend /build/internal/webui/static ./internal/webui/static
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -trimpath -o /app ./cmd/server

# Stage 3: Runtime
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=backend /app /app
USER 65532:65532
EXPOSE 8080 8081
ENTRYPOINT ["/app"]
