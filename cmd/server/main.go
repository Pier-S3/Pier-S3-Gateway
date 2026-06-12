package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"pier-s3/internal/auth"
	"pier-s3/internal/config"
	"pier-s3/internal/proxy"
	"pier-s3/internal/webui"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Fail fast at startup if any required configuration is missing rather
	// than starting with empty credentials and surfacing the failure later.
	cfg, err := config.LoadAndValidate()
	if err != nil {
		logger.Error("invalid configuration", "error", err.Error())
		os.Exit(1)
	}

	// Resolve provider-neutral OIDC settings (OIDC_* over KEYCLOAK_* fallbacks,
	// plus optional discovery) so any compliant OIDC IdP works by configuration.
	oidcSettings, err := cfg.ResolveOIDC()
	if err != nil {
		logger.Error("failed to resolve OIDC settings", "error", err.Error())
		os.Exit(1)
	}

	oidcCfg := auth.NewOIDCConfig(
		cfg.KeycloakURL,
		cfg.KeycloakRealm,
		oidcSettings.ClientID,
		cfg.KeycloakClientSecret,
		// Explicit redirect_uri (OIDC_REDIRECT_URI); allowlist the exact value
		// at the IdP. Empty keeps the legacy behavior where the frontend
		// supplies it via oidc-client-ts.
		cfg.OIDCRedirectURI,
	)

	// Claim mapping: empty username/groups claims select the default
	// (Keycloak-compatible) extraction.
	claimMapper := auth.ClaimMapper{
		UsernameClaim: oidcSettings.UsernameClaim,
		GroupsClaim:   oidcSettings.GroupsClaim,
	}

	// Bind tokens to this issuer and audience so a validly signed token minted
	// for a different realm or client cannot authenticate.
	verifier, err := auth.NewJWKSVerifier(oidcSettings.JWKSURL, oidcSettings.Issuer, oidcSettings.Audience)
	if err != nil {
		logger.Error("failed to initialize JWKS verifier", "error", err.Error())
		os.Exit(1)
	}
	defer verifier.Close()

	s3Client, err := proxy.NewS3Client(cfg.S3Endpoint, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Region)
	if err != nil {
		logger.Error("failed to initialize S3 client", "error", err.Error())
		os.Exit(1)
	}

	// Display-only bucket quotas for the UI stats endpoint; a malformed
	// BUCKET_QUOTAS is a startup error rather than a silently missing quota.
	quotas, err := cfg.ParseBucketQuotas()
	if err != nil {
		logger.Error("invalid bucket quotas", "error", err.Error())
		os.Exit(1)
	}

	s3Server := buildS3Server(cfg, verifier, s3Client, claimMapper, logger)
	uiServer := buildUIServer(cfg, verifier, s3Client, oidcCfg, claimMapper, quotas, logger)
	healthServer := buildHealthServer(cfg)

	var wg sync.WaitGroup
	wg.Add(3)
	go serve(s3Server, "S3 proxy server", logger, &wg)
	go serve(uiServer, "Web UI server", logger, &wg)
	go serve(healthServer, "health server", logger, &wg)

	waitForShutdownSignal(logger)
	gracefulShutdown([]*http.Server{s3Server, uiServer, healthServer}, logger)
	wg.Wait()
	logger.Info("shutdown complete")
}

func buildS3Server(cfg *config.Config, verifier auth.TokenVerifier, s3Client *proxy.S3Client, mapper auth.ClaimMapper, logger *slog.Logger) *http.Server {
	s3Handler := proxy.NewS3HandlerWithMapper(verifier, s3Client, mapper, logger)
	mux := http.NewServeMux()
	mux.Handle("/", s3Handler)

	return &http.Server{
		Addr:              cfg.ListenS3Addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		// WriteTimeout is intentionally 0 (disabled): this server streams
		// arbitrary-size S3 objects, and a fixed write deadline would abort
		// legitimate large/slow transfers. Slow-read exhaustion is instead
		// bounded by ReadHeaderTimeout, IdleTimeout, the ingress
		// proxy-send/read timeouts, and per-request context cancellation when
		// the client disconnects.
		WriteTimeout: 0,
	}
}

func buildUIServer(cfg *config.Config, verifier auth.TokenVerifier, s3Client *proxy.S3Client, oidcCfg *auth.OIDCConfig, mapper auth.ClaimMapper, quotas map[string]int64, logger *slog.Logger) *http.Server {
	mux := http.NewServeMux()

	api := webui.NewAPI(s3Client, logger)
	api.SetBucketQuotas(quotas)
	api.RegisterRoutes(mux, verifier, oidcCfg, mapper)
	mux.Handle("/", webui.StaticHandler(cfg.KeycloakURL))

	return &http.Server{
		Addr:              cfg.ListenUIAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		// WriteTimeout is intentionally 0 (disabled): the UI API also streams
		// object downloads/uploads of arbitrary size, so a fixed write
		// deadline would abort legitimate transfers. Abuse is bounded by the
		// per-IP rate limiter on auth endpoints, ReadHeaderTimeout,
		// IdleTimeout, and the ingress timeouts.
		WriteTimeout: 0,
	}
}

// buildHealthServer serves /_health and /_ready on a dedicated listener that
// is intentionally NOT part of the s3/ui servers: the k8s Service and Ingress
// only expose those, so the probe endpoints are never reachable from outside
// the cluster - the kubelet probes this port directly.
func buildHealthServer(cfg *config.Config) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/_health", healthHandler)
	mux.HandleFunc("/_ready", readyHandler)

	return &http.Server{
		Addr:              cfg.ListenHealthAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		// Tiny fixed responses only - a write deadline is safe here.
		WriteTimeout: 10 * time.Second,
	}
}

func serve(server *http.Server, name string, logger *slog.Logger, wg *sync.WaitGroup) {
	defer wg.Done()
	logger.Info("starting "+name, "addr", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error(name+" error", "error", err.Error())
	}
}

func waitForShutdownSignal(logger *slog.Logger) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	logger.Info("received signal, shutting down", "signal", sig.String())
}

func gracefulShutdown(servers []*http.Server, logger *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(len(servers))
	for _, s := range servers {
		go func(srv *http.Server) {
			defer wg.Done()
			if err := srv.Shutdown(ctx); err != nil {
				logger.Error("server shutdown error", "addr", srv.Addr, "error", err.Error())
			}
		}(s)
	}
	wg.Wait()
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func readyHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ready"}`))
}
