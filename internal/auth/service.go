package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"future-voice/internal/features/user"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

const (
	telegramIssuer = "https://oauth.telegram.org"
	sessionCookie  = "fv_session"
	csrfCookie     = "fv_csrf"
	stateCookie    = "fv_oidc_state"
	principalKey   = "admin_principal"
)

var (
	ErrNotAuthenticated = errors.New("not authenticated")
	ErrForbidden        = errors.New("not authorized for the admin panel")
)

type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AppURL       string
	CookieSecure bool
	SessionTTL   time.Duration
}

type Principal struct {
	ID         int64  `json:"id"`
	TelegramID int64  `json:"telegram_id"`
	Role       string `json:"role"`
	Username   string `json:"username"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
}

type Service struct {
	db         *sql.DB
	users      userReader
	oauth      oauth2.Config
	verifier   *oidc.IDTokenVerifier
	appURL     string
	secure     bool
	sessionTTL time.Duration
}

type userReader interface {
	GetByTelegramID(ctx context.Context, telegramID int64) (*user.User, error)
}

type authFlow struct {
	Nonce        string
	CodeVerifier string
}

func NewService(ctx context.Context, db *sql.DB, users userReader, cfg Config) (*Service, error) {
	appURL, err := url.Parse(strings.TrimRight(cfg.AppURL, "/"))
	if err != nil || appURL.Scheme == "" || appURL.Host == "" {
		return nil, errors.New("admin app URL must be an absolute URL")
	}
	if cfg.SessionTTL <= 0 {
		cfg.SessionTTL = 8 * time.Hour
	}

	s := &Service{
		db:         db,
		users:      users,
		appURL:     strings.TrimRight(cfg.AppURL, "/"),
		secure:     cfg.CookieSecure,
		sessionTTL: cfg.SessionTTL,
	}

	if cfg.ClientID == "" && cfg.ClientSecret == "" && cfg.RedirectURL == "" {
		return s, nil
	}
	if cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.RedirectURL == "" {
		return nil, errors.New("Telegram OIDC configuration is incomplete")
	}

	discoveryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	provider, err := oidc.NewProvider(discoveryCtx, telegramIssuer)
	if err != nil {
		return nil, fmt.Errorf("discover Telegram OIDC provider: %w", err)
	}
	endpoint := provider.Endpoint()
	endpoint.AuthStyle = oauth2.AuthStyleInHeader

	s.oauth = oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint:     endpoint,
		RedirectURL:  cfg.RedirectURL,
		Scopes:       []string{oidc.ScopeOpenID, "profile"},
	}
	s.verifier = provider.Verifier(&oidc.Config{ClientID: cfg.ClientID})

	return s, nil
}

func (s *Service) Configured() bool {
	return s.verifier != nil
}

func (s *Service) Start(c *gin.Context) {
	if !s.Configured() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Telegram OIDC is not configured"})
		return
	}

	state, err := randomToken(32)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not start login"})
		return
	}
	nonce, err := randomToken(32)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not start login"})
		return
	}
	verifier := oauth2.GenerateVerifier()
	expiresAt := time.Now().Add(10 * time.Minute)

	if _, err := s.db.ExecContext(c.Request.Context(), `
		DELETE FROM admin_auth_flows WHERE expires_at <= NOW()
	`); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not start login"})
		return
	}
	_, err = s.db.ExecContext(c.Request.Context(), `
		INSERT INTO admin_auth_flows (state_hash, nonce, code_verifier, expires_at)
		VALUES ($1, $2, $3, $4)
	`, tokenHash(state), nonce, verifier, expiresAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not start login"})
		return
	}

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     stateCookie,
		Value:    state,
		Path:     "/api/v1/auth/telegram/callback",
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
		Expires:  expiresAt,
	})

	authURL := s.oauth.AuthCodeURL(
		state,
		oauth2.SetAuthURLParam("nonce", nonce),
		oauth2.S256ChallengeOption(verifier),
	)
	c.Redirect(http.StatusFound, authURL)
}

func (s *Service) Callback(c *gin.Context) {
	if !s.Configured() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Telegram OIDC is not configured"})
		return
	}
	if providerError := c.Query("error"); providerError != "" {
		s.clearStateCookie(c.Writer)
		c.Redirect(http.StatusFound, s.appURL+"/login?error=telegram_login_failed")
		return
	}

	state := c.Query("state")
	stateCookieValue, cookieErr := c.Cookie(stateCookie)
	if state == "" || cookieErr != nil ||
		subtle.ConstantTimeCompare([]byte(state), []byte(stateCookieValue)) != 1 {
		s.clearStateCookie(c.Writer)
		c.Redirect(http.StatusFound, s.appURL+"/login?error=invalid_login_state")
		return
	}
	if c.Query("code") == "" {
		s.clearStateCookie(c.Writer)
		c.Redirect(http.StatusFound, s.appURL+"/login?error=missing_authorization_code")
		return
	}

	flow, err := s.consumeFlow(c.Request.Context(), state)
	s.clearStateCookie(c.Writer)
	if err != nil {
		c.Redirect(http.StatusFound, s.appURL+"/login?error=expired_login")
		return
	}

	token, err := s.oauth.Exchange(
		c.Request.Context(),
		c.Query("code"),
		oauth2.VerifierOption(flow.CodeVerifier),
	)
	if err != nil {
		c.Redirect(http.StatusFound, s.appURL+"/login?error=telegram_token_exchange_failed")
		return
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		c.Redirect(http.StatusFound, s.appURL+"/login?error=missing_id_token")
		return
	}

	idToken, err := s.verifier.Verify(c.Request.Context(), rawIDToken)
	if err != nil {
		c.Redirect(http.StatusFound, s.appURL+"/login?error=invalid_id_token")
		return
	}
	if idToken.Nonce == "" || subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(flow.Nonce)) != 1 {
		c.Redirect(http.StatusFound, s.appURL+"/login?error=invalid_login_nonce")
		return
	}

	var claims map[string]json.RawMessage
	if err := idToken.Claims(&claims); err != nil {
		slog.Error("decode Telegram OIDC claims", "error", err)
		c.Redirect(http.StatusFound, s.appURL+"/login?error=invalid_telegram_profile")
		return
	}
	telegramID, err := parseTelegramIDClaim(claims["id"])
	if err != nil || idToken.Subject == "" {
		// Do not log the ID token, subject, or profile claims: these are
		// authentication data. Presence flags are enough to diagnose this case.
		slog.Warn(
			"Telegram OIDC token does not contain a usable profile",
			"id_claim_present", len(claims["id"]) > 0,
			"subject_present", idToken.Subject != "",
			"id_claim_error", err,
		)
		c.Redirect(http.StatusFound, s.appURL+"/login?error=invalid_telegram_profile")
		return
	}

	account, err := s.users.GetByTelegramID(c.Request.Context(), telegramID)
	if err != nil || account == nil || !account.IsActive {
		c.Redirect(http.StatusFound, s.appURL+"/login?error=account_not_allowed")
		return
	}
	if account.Role != user.AdminRole && account.Role != user.ModeratorRole {
		c.Redirect(http.StatusFound, s.appURL+"/login?error=account_not_allowed")
		return
	}

	sessionToken, err := randomToken(32)
	if err != nil {
		c.Redirect(http.StatusFound, s.appURL+"/login?error=session_creation_failed")
		return
	}
	csrfToken, err := randomToken(32)
	if err != nil {
		c.Redirect(http.StatusFound, s.appURL+"/login?error=session_creation_failed")
		return
	}
	expiresAt := time.Now().Add(s.sessionTTL)
	if _, err := s.db.ExecContext(c.Request.Context(), `
		DELETE FROM admin_sessions
		WHERE expires_at <= NOW()
		   OR revoked_at < NOW() - INTERVAL '7 days'
	`); err != nil {
		c.Redirect(http.StatusFound, s.appURL+"/login?error=session_creation_failed")
		return
	}
	_, err = s.db.ExecContext(c.Request.Context(), `
		INSERT INTO admin_sessions (token_hash, csrf_hash, user_id, expires_at)
		VALUES ($1, $2, $3, $4)
	`, tokenHash(sessionToken), tokenHash(csrfToken), account.ID, expiresAt)
	if err != nil {
		c.Redirect(http.StatusFound, s.appURL+"/login?error=session_creation_failed")
		return
	}

	s.setSessionCookies(c.Writer, sessionToken, csrfToken, expiresAt)
	c.Redirect(http.StatusFound, s.appURL+"/")
}

func (s *Service) Me(c *gin.Context) {
	principal, ok := PrincipalFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": principal})
}

func (s *Service) Logout(c *gin.Context) {
	cookie, err := c.Request.Cookie(sessionCookie)
	if err == nil && cookie.Value != "" {
		_, _ = s.db.ExecContext(
			c.Request.Context(),
			`UPDATE admin_sessions SET revoked_at = NOW() WHERE token_hash = $1 AND revoked_at IS NULL`,
			tokenHash(cookie.Value),
		)
	}
	s.clearSessionCookies(c.Writer)
	c.Status(http.StatusNoContent)
}

func (s *Service) RequireStaff() gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Request.Cookie(sessionCookie)
		if err != nil || cookie.Value == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var principal Principal
		var csrfHash []byte
		err = s.db.QueryRowContext(c.Request.Context(), `
			SELECT u.id, u.telegram_id, u.role, u.username, u.first_name, u.last_name, s.csrf_hash
			FROM admin_sessions s
			JOIN users u ON u.id = s.user_id
			WHERE s.token_hash = $1
			  AND s.revoked_at IS NULL
			  AND s.expires_at > NOW()
			  AND u.is_active = TRUE
		`, tokenHash(cookie.Value)).Scan(
			&principal.ID,
			&principal.TelegramID,
			&principal.Role,
			&principal.Username,
			&principal.FirstName,
			&principal.LastName,
			&csrfHash,
		)
		if errors.Is(err, sql.ErrNoRows) {
			s.clearSessionCookies(c.Writer)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "session expired"})
			return
		}
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "could not validate session"})
			return
		}
		if principal.Role != string(user.AdminRole) && principal.Role != string(user.ModeratorRole) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}

		if requiresCSRF(c.Request.Method) {
			csrfCookie, cookieErr := c.Request.Cookie(csrfCookie)
			csrfHeader := c.GetHeader("X-CSRF-Token")
			if cookieErr != nil || csrfCookie.Value == "" || csrfHeader == "" ||
				subtle.ConstantTimeCompare([]byte(csrfCookie.Value), []byte(csrfHeader)) != 1 ||
				subtle.ConstantTimeCompare(tokenHash(csrfHeader), csrfHash) != 1 {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid CSRF token"})
				return
			}
		}

		c.Set(principalKey, principal)
		c.Set("user_id", principal.ID)
		c.Set("telegram_id", principal.TelegramID)
		c.Set("role", principal.Role)
		c.Next()
	}
}

func PrincipalFromContext(c *gin.Context) (Principal, bool) {
	value, ok := c.Get(principalKey)
	if !ok {
		return Principal{}, false
	}
	principal, ok := value.(Principal)
	return principal, ok
}

func (s *Service) consumeFlow(ctx context.Context, state string) (authFlow, error) {
	var flow authFlow
	err := s.db.QueryRowContext(ctx, `
		DELETE FROM admin_auth_flows
		WHERE state_hash = $1 AND expires_at > NOW()
		RETURNING nonce, code_verifier
	`, tokenHash(state)).Scan(&flow.Nonce, &flow.CodeVerifier)
	if err != nil {
		return authFlow{}, fmt.Errorf("consume OIDC flow: %w", err)
	}
	return flow, nil
}

func (s *Service) setSessionCookies(w http.ResponseWriter, session, csrf string, expiresAt time.Time) {
	maxAge := int(time.Until(expiresAt).Seconds())
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: session, Path: "/", HttpOnly: true,
		Secure: s.secure, SameSite: http.SameSiteLaxMode, MaxAge: maxAge, Expires: expiresAt,
	})
	http.SetCookie(w, &http.Cookie{
		Name: csrfCookie, Value: csrf, Path: "/", HttpOnly: false,
		Secure: s.secure, SameSite: http.SameSiteLaxMode, MaxAge: maxAge, Expires: expiresAt,
	})
}

func (s *Service) clearStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: stateCookie, Value: "", Path: "/api/v1/auth/telegram/callback",
		HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode,
		MaxAge: -1, Expires: time.Unix(1, 0),
	})
}

func (s *Service) clearSessionCookies(w http.ResponseWriter) {
	for _, name := range []string{sessionCookie, csrfCookie} {
		http.SetCookie(w, &http.Cookie{
			Name: name, Value: "", Path: "/", HttpOnly: name == sessionCookie,
			Secure: s.secure, SameSite: http.SameSiteLaxMode,
			MaxAge: -1, Expires: time.Unix(1, 0),
		})
	}
}

func tokenHash(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func randomToken(size int) (string, error) {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func requiresCSRF(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

// parseTelegramIDClaim reads Telegram's profile "id" claim. Telegram documents
// it as a number; accepting a decimal JSON string as well makes the boundary
// resilient to provider serialization differences without trusting the OIDC
// subject as a substitute (sub and Telegram's numeric id are distinct claims).
func parseTelegramIDClaim(raw json.RawMessage) (int64, error) {
	if len(raw) == 0 {
		return 0, errors.New("Telegram profile id claim is missing")
	}

	var id int64
	if err := json.Unmarshal(raw, &id); err == nil {
		if id > 0 {
			return id, nil
		}
		return 0, errors.New("Telegram profile id must be positive")
	}

	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, fmt.Errorf("Telegram profile id has unsupported JSON type: %w", err)
	}
	id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("Telegram profile id is not a positive integer")
	}
	return id, nil
}

// ParseTelegramID is kept small and explicit for tests and diagnostics.
func ParseTelegramID(value string) (int64, error) {
	return strconv.ParseInt(value, 10, 64)
}

func HashForTests(value string) string {
	return hex.EncodeToString(tokenHash(value))
}
