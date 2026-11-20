package main

import (
	"context"
	"errors"
	"fmt"
	httpapi "future-voice/cmd/http"
	"future-voice/internal/auth"
	"future-voice/internal/config"
	"future-voice/internal/features/analytics"
	requestFeature "future-voice/internal/features/request"
	userFeature "future-voice/internal/features/user"
	"future-voice/internal/observability/metrics"
	"future-voice/internal/storage"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		slog.Error("HTTP server stopped", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	if err := godotenv.Load(); err != nil {
		slog.Info("No .env file found; reading environment variables")
	}

	cfg, err := config.LoadServer()
	if err != nil {
		return fmt.Errorf("load server config: %w", err)
	}

	store, err := storage.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("initialize storage: %w", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			slog.Error("close database", "error", err)
		}
	}()

	userRepository := userFeature.New(store.DB)
	userUseCase := userFeature.NewUseCase(userRepository)

	requestRepository := requestFeature.New(store.DB)
	requestUseCase := requestFeature.NewUseCase(requestRepository, userUseCase)

	authConfig := auth.Config{
		ClientID:     cfg.TelegramOIDCClientID,
		ClientSecret: cfg.TelegramOIDCClientSecret,
		RedirectURL:  cfg.TelegramOIDCRedirectURL,
		AppURL:       cfg.AdminWebURL,
		CookieSecure: cfg.CookieSecure,
		SessionTTL:   cfg.SessionTTL,
	}
	authService, authErr := auth.NewService(ctx, store.DB, userUseCase, authConfig)
	if authErr != nil {
		// OIDC discovery requires Telegram's identity provider to be reachable.
		// Keep the request backend available when that external dependency is down;
		// the web login endpoint will return 503 until the server is restarted.
		slog.Warn("Telegram OIDC is unavailable; web login is disabled", "error", authErr)
		authService, err = auth.NewService(ctx, store.DB, userUseCase, auth.Config{
			AppURL:       cfg.AdminWebURL,
			CookieSecure: cfg.CookieSecure,
			SessionTTL:   cfg.SessionTTL,
		})
		if err != nil {
			return fmt.Errorf("initialize fallback admin authentication: %w", err)
		}
	}
	if !authService.Configured() {
		slog.Warn("Telegram OIDC is not configured; new web admin logins are disabled")
	}

	appMetrics := metrics.New(store.DB)
	analyticsHandler := analytics.NewHandler(analytics.NewRepository(store.DB))

	addr := strings.TrimSpace(cfg.Port)
	if !strings.Contains(addr, ":") {
		addr = ":" + addr
	}

	server := &http.Server{
		Addr: addr,
		Handler: httpapi.NewRouter(httpapi.Dependencies{
			BotAPISecret:    cfg.BotAPISecret,
			UserUseCase:     userUseCase,
			RequestUseCase:  requestUseCase,
			Auth:            authService,
			Analytics:       analyticsHandler,
			Metrics:         appMetrics,
			MetricsUser:     cfg.MetricsUser,
			MetricsPassword: cfg.MetricsPassword,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		slog.Info("HTTP server starting", "addr", addr)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("listen and serve: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("graceful HTTP shutdown: %w", err)
		}
		if err := <-serverErrors; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("HTTP server: %w", err)
		}
		return nil
	}
}
