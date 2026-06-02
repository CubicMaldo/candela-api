package web

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tuusuario/nomad-api/internal/handler"
	"github.com/tuusuario/nomad-api/internal/repo"
	"github.com/tuusuario/nomad-api/internal/service"
	"github.com/tuusuario/nomad-api/internal/web/middleware"
	"github.com/tuusuario/nomad-api/internal/webdeps"
)

func NewRouter(deps webdeps.Dependencies) *gin.Engine {
	if deps.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()
	router.GET("/health", healthHandler(deps.Env))

	// Inicializar capas
	userRepo := repo.NewUserRepository(deps.DBPool)
	authService := service.NewAuthService(userRepo, deps.Config)
	authHandler := handler.NewAuthHandler(authService)

	// Rutas
	api := router.Group("/api")
	{
		// Rutas públicas
		auth := api.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.POST("/google", authHandler.GoogleLogin)
		}

		// Rutas protegidas (ejemplos futuros)
		protected := api.Group("")
		protected.Use(middleware.AuthMiddleware(deps.Config.JWTSecret))
		{
			// Aquí irán /places, /checkins, etc.
			protected.GET("/me", func(c *gin.Context) {
				userID := c.MustGet("userID").(string)
				c.JSON(200, gin.H{"user_id": userID, "message": "Estás autenticado"})
			})
		}
	}

	return router
}

func healthHandler(env string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "nomad-api",
			"env":     env,
		})
	}
}