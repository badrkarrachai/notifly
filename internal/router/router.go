package router

import (
	"net/http"

	"notifly/internal/common"
	"notifly/internal/config"
	"notifly/internal/domain/notification"
	"notifly/internal/middleware"

	"github.com/gin-gonic/gin"
)

// New creates and configures the Gin router with all middleware and routes.
func New(
	cfg *config.Config,
	notificationHandler *notification.Handler,
) *gin.Engine {
	// Set Gin mode
	gin.SetMode(cfg.Server.Mode)

	r := gin.New()

	// Global middleware stack (order matters)
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.CORS(
		cfg.CORS.AllowedOrigins,
		cfg.CORS.AllowedMethods,
		cfg.CORS.AllowedHeaders,
	))

	// Rate limiter
	rateLimiter := middleware.NewRateLimiter(
		cfg.RateLimit.RequestsPerSecond,
		cfg.RateLimit.Burst,
	)
	r.Use(rateLimiter.Middleware())

	// Custom structured logger middleware
	r.Use(gin.Logger())

	// Public routes
	r.GET("/health", healthCheck)

	// Protected API routes (API key required)
	protectedAPI := r.Group("/api/v1")
	protectedAPI.Use(middleware.Auth(cfg.Auth.APIKeys))
	{
		notificationHandler.RegisterRoutes(protectedAPI)
	}

	return r
}

// healthCheck handles GET /health
func healthCheck(c *gin.Context) {
	common.Success(c, http.StatusOK, gin.H{
		"status":  "ok",
		"service": "notifly",
	})
}
