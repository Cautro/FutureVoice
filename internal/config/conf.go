package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config contains process configuration. LoadServer and LoadTelegram keep
// each process limited to the settings it actually needs.
type Config struct {
	Port         string
	DatabaseURL  string
	BotToken     string
	BackendURL   string
	BotAPISecret string

	TelegramOIDCClientID     string
	TelegramOIDCClientSecret string
	TelegramOIDCRedirectURL  string
	AdminWebURL              string
	SessionTTL               time.Duration
	CookieSecure             bool

	MetricsUser     string
	MetricsPassword string
}

func LoadServer() (Config, error) {
	port := envOrDefault("PORT", "8080")
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		databaseURL = strings.TrimSpace(os.Getenv("DB_PATH"))
	}
	if databaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}

	botSecret := strings.TrimSpace(os.Getenv("BOT_API_SECRET"))
	if botSecret == "" {
		return Config{}, errors.New("BOT_API_SECRET is required")
	}

	oidcClientID := strings.TrimSpace(os.Getenv("TELEGRAM_OIDC_CLIENT_ID"))
	oidcClientSecret := strings.TrimSpace(os.Getenv("TELEGRAM_OIDC_CLIENT_SECRET"))
	oidcRedirectURL := strings.TrimSpace(os.Getenv("TELEGRAM_OIDC_REDIRECT_URL"))
	if (oidcClientID != "" || oidcClientSecret != "" || oidcRedirectURL != "") &&
		(oidcClientID == "" || oidcClientSecret == "" || oidcRedirectURL == "") {
		return Config{}, errors.New("TELEGRAM_OIDC_CLIENT_ID, TELEGRAM_OIDC_CLIENT_SECRET and TELEGRAM_OIDC_REDIRECT_URL must be configured together")
	}

	sessionTTL := 8 * time.Hour
	if raw := strings.TrimSpace(os.Getenv("ADMIN_SESSION_TTL")); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil || parsed < time.Minute || parsed > 7*24*time.Hour {
			return Config{}, errors.New("ADMIN_SESSION_TTL must be between 1m and 168h")
		}
		sessionTTL = parsed
	}

	metricsUser := strings.TrimSpace(os.Getenv("METRICS_USER"))
	metricsPassword := strings.TrimSpace(os.Getenv("METRICS_PASSWORD"))
	if (metricsUser == "") != (metricsPassword == "") {
		return Config{}, errors.New("METRICS_USER and METRICS_PASSWORD must both be configured or both be empty")
	}

	adminWebURL := strings.TrimRight(envOrDefault("ADMIN_WEB_URL", "http://localhost:5173"), "/")
	parsedAdminURL, err := url.Parse(adminWebURL)
	if err != nil || (parsedAdminURL.Scheme != "http" && parsedAdminURL.Scheme != "https") || parsedAdminURL.Host == "" {
		return Config{}, errors.New("ADMIN_WEB_URL must be an absolute HTTP or HTTPS URL")
	}
	cookieSecure, err := envBoolStrict("COOKIE_SECURE", false)
	if err != nil {
		return Config{}, err
	}
	localHost := isLocalHost(parsedAdminURL.Hostname())
	if !localHost && parsedAdminURL.Scheme != "https" {
		return Config{}, errors.New("ADMIN_WEB_URL must use HTTPS outside localhost")
	}
	if !localHost && !cookieSecure {
		return Config{}, errors.New("COOKIE_SECURE=true is required outside localhost")
	}
	if oidcRedirectURL != "" {
		parsedRedirectURL, parseErr := url.Parse(oidcRedirectURL)
		if parseErr != nil || parsedRedirectURL.Host == "" || parsedRedirectURL.Scheme != parsedAdminURL.Scheme || !strings.EqualFold(parsedRedirectURL.Host, parsedAdminURL.Host) {
			return Config{}, errors.New("TELEGRAM_OIDC_REDIRECT_URL must use the same origin as ADMIN_WEB_URL")
		}
		if parsedRedirectURL.Path != "/api/v1/auth/telegram/callback" {
			return Config{}, errors.New("TELEGRAM_OIDC_REDIRECT_URL must end with /api/v1/auth/telegram/callback")
		}
		if !localHost && parsedRedirectURL.Scheme != "https" {
			return Config{}, errors.New("Telegram OIDC callback must use HTTPS outside localhost")
		}
	}

	return Config{
		Port:                     port,
		DatabaseURL:              databaseURL,
		BotAPISecret:             botSecret,
		TelegramOIDCClientID:     oidcClientID,
		TelegramOIDCClientSecret: oidcClientSecret,
		TelegramOIDCRedirectURL:  oidcRedirectURL,
		AdminWebURL:              adminWebURL,
		SessionTTL:               sessionTTL,
		CookieSecure:             cookieSecure,
		MetricsUser:              metricsUser,
		MetricsPassword:          metricsPassword,
	}, nil
}

func LoadTelegram() (Config, error) {
	token := strings.TrimSpace(os.Getenv("BOT_TOKEN"))
	if token == "" {
		return Config{}, errors.New("BOT_TOKEN is required")
	}

	backendURL := strings.TrimRight(strings.TrimSpace(os.Getenv("BACKEND_URL")), "/")
	if backendURL == "" {
		backendURL = "http://127.0.0.1:8080"
	}
	if !strings.HasPrefix(backendURL, "http://") && !strings.HasPrefix(backendURL, "https://") {
		return Config{}, fmt.Errorf("BACKEND_URL must start with http:// or https://")
	}

	secret := strings.TrimSpace(os.Getenv("BOT_API_SECRET"))
	if secret == "" {
		return Config{}, errors.New("BOT_API_SECRET is required")
	}

	return Config{
		BotToken:     token,
		BackendURL:   backendURL,
		BotAPISecret: secret,
	}, nil
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envBoolStrict(key string, fallback bool) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", key, err)
	}
	return value, nil
}

func isLocalHost(host string) bool {
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}
