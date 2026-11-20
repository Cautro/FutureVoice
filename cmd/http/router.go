package http

import (
	"crypto/subtle"
	"net/http"

	"future-voice/internal/auth"
	"future-voice/internal/features/analytics"
	requestFeature "future-voice/internal/features/request"
	userFeature "future-voice/internal/features/user"
	"future-voice/internal/observability/metrics"

	"github.com/gin-gonic/gin"
)

type Dependencies struct {
	BotAPISecret    string
	UserUseCase     *userFeature.UserUseCase
	RequestUseCase  *requestFeature.RequestUseCase
	Auth            *auth.Service
	Analytics       *analytics.Handler
	Metrics         *metrics.Metrics
	MetricsUser     string
	MetricsPassword string
}

func NewRouter(deps Dependencies) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery(), gin.Logger())
	if deps.Metrics != nil {
		router.Use(deps.Metrics.Middleware())
	}

	registerGeneralRoutes(router, deps)
	registerTelegramRoutes(deps, router)
	registerAuthRoutes(deps, router)
	registerAdminRoutes(deps, router)
	return router
}

func registerGeneralRoutes(router *gin.Engine, deps Dependencies) {
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	if deps.Metrics != nil && deps.MetricsUser != "" && deps.MetricsPassword != "" {
		router.GET("/metrics", gin.WrapH(deps.Metrics.Handler(deps.MetricsUser, deps.MetricsPassword)))
	}
}

func registerTelegramRoutes(deps Dependencies, router *gin.Engine) {
	telegram := router.Group("/api/v1/internal/telegram")
	telegram.Use(requireBotSecret(deps.BotAPISecret))
	{
		telegram.POST("/users", userFeature.RegisterTelegramUserHandler(deps.UserUseCase))
		telegram.POST("/requests", requestFeature.CreateHandler(deps.RequestUseCase))
		telegram.GET("/requests", requestFeature.ListHandler(deps.RequestUseCase))
		telegram.GET("/requests/:id", requestFeature.StudentGetHandler(deps.RequestUseCase))
		telegram.GET("/requests/:id/messages", requestFeature.StudentMessagesHandler(deps.RequestUseCase))
		telegram.POST("/requests/:id/messages", requestFeature.StudentAddMessageHandler(deps.RequestUseCase))
		telegram.GET("/notifications", requestFeature.TelegramNotificationsHandler(deps.RequestUseCase))
		telegram.POST("/notifications/:id/delivered", requestFeature.TelegramNotificationDeliveredHandler(deps.RequestUseCase))
		telegram.POST("/notifications/:id/failed", requestFeature.TelegramNotificationFailedHandler(deps.RequestUseCase))
		telegram.DELETE("/requests/:id", requestFeature.DeleteHandler(deps.RequestUseCase))
		telegram.PATCH("/requests/:id/status", requestFeature.UpdateStatusHandler(deps.RequestUseCase))
	}
}

func registerAuthRoutes(deps Dependencies, router *gin.Engine) {
	authRoutes := router.Group("/api/v1/auth")
	if deps.Auth == nil {
		authRoutes.GET("/telegram/start", serviceUnavailable)
		authRoutes.GET("/telegram/callback", serviceUnavailable)
		authRoutes.GET("/me", serviceUnavailable)
		authRoutes.POST("/logout", serviceUnavailable)
		return
	}
	authRoutes.GET("/telegram/start", deps.Auth.Start)
	authRoutes.GET("/telegram/callback", deps.Auth.Callback)
	authRoutes.GET("/me", deps.Auth.RequireStaff(), deps.Auth.Me)
	authRoutes.POST("/logout", deps.Auth.RequireStaff(), deps.Auth.Logout)
}

func registerAdminRoutes(deps Dependencies, router *gin.Engine) {
	if deps.Auth == nil {
		return
	}
	admin := router.Group("/api/v1/admin")
	admin.Use(deps.Auth.RequireStaff())
	{
		admin.GET("/requests", requestFeature.AdminListHandler(deps.RequestUseCase))
		admin.GET("/requests/:id", requestFeature.AdminGetHandler(deps.RequestUseCase))
		admin.PATCH("/requests/:id/status", requestFeature.AdminUpdateStatusHandler(deps.RequestUseCase))
		admin.PATCH("/requests/:id/assignee", requestFeature.AdminAssignHandler(deps.RequestUseCase))
		admin.POST("/requests/:id/comments", requestFeature.AdminCommentHandler(deps.RequestUseCase))
		admin.POST("/requests/:id/messages", requestFeature.AdminMessageHandler(deps.RequestUseCase))
		admin.DELETE("/requests/:id", requestFeature.AdminDeleteHandler(deps.RequestUseCase))
		admin.GET("/users", requireAdmin(), userFeature.AdminListHandler(deps.UserUseCase))
		admin.PATCH("/users/:id/role", requireAdmin(), userFeature.AdminRoleHandler(deps.UserUseCase))
		admin.PATCH("/users/:id/active", requireAdmin(), userFeature.AdminActiveHandler(deps.UserUseCase))
		admin.GET("/analytics/overview", requireAdmin(), deps.Analytics.Overview)
	}
}

func requireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString("role") != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "administrator role required"})
			return
		}
		c.Next()
	}
}

func serviceUnavailable(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "authentication is not configured"})
}

func requireBotSecret(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if secret == "" {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "bot integration is not configured"})
			return
		}

		provided := c.GetHeader("X-Bot-Secret")
		if len(provided) != len(secret) ||
			subtle.ConstantTimeCompare([]byte(provided), []byte(secret)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
		c.Next()
	}
}
