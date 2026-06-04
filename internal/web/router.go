package web

import (
	"time"

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

	authLimiter := middleware.RateLimiter(deps.Redis, 10, time.Minute)

	// Repositories
	userRepo := repo.NewUserRepository(deps.DBPool)
	placeRepo := repo.NewPlaceRepository(deps.DBPool)
	checkinRepo := repo.NewCheckInRepository(deps.DBPool)
	rankingRepo := repo.NewRankingRepository(deps.DBPool)
	friendshipRepo := repo.NewFriendshipRepository(deps.DBPool)
	challengeRepo := repo.NewChallengeRepository(deps.DBPool)
	feedRepo := repo.NewFeedRepository(deps.DBPool)

	// Services
	authService := service.NewAuthService(userRepo, deps.Config)
	placeService := service.NewPlaceService(placeRepo)
	checkinService := service.NewCheckInService(checkinRepo, challengeRepo, deps.Storage, deps.Redis)
	rankingService := service.NewRankingService(rankingRepo, deps.Redis)
	userService := service.NewUserService(userRepo, friendshipRepo, deps.Redis)
	friendService := service.NewFriendService(friendshipRepo, deps.Redis)
	challengeService := service.NewChallengeService(challengeRepo, friendshipRepo, placeRepo)
	profileService := service.NewProfileService(userRepo, feedRepo, checkinRepo, challengeRepo)

	// Handlers
	authHandler := handler.NewAuthHandler(authService)
	placeHandler := handler.NewPlaceHandler(placeService)
	checkinHandler := handler.NewCheckInHandler(checkinService)
	rankingHandler := handler.NewRankingHandler(rankingService)
	userHandler := handler.NewUserHandler(userService)
	friendHandler := handler.NewFriendHandler(friendService)
	challengeHandler := handler.NewChallengeHandler(challengeService)
	profileHandler := handler.NewProfileHandler(profileService)

	api := router.Group("/api")
	{
		auth := api.Group("/auth")
		auth.Use(authLimiter)
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.POST("/google", authHandler.GoogleLogin)
		}

		protected := api.Group("")
		protected.Use(middleware.AuthMiddleware(deps.Config.JWTSecret, deps.Redis))
		{
			protected.GET("/me", profileHandler.GetMe)
			protected.PATCH("/me/privacy", userHandler.UpdatePrivacy)
			protected.DELETE("/me", userHandler.DeleteAccount)
			protected.GET("/me/friends", friendHandler.GetFriends)
			protected.GET("/me/check-ins", profileHandler.GetMyCheckIns)
			protected.GET("/me/challenges", profileHandler.GetMyChallenges)

			protected.GET("/feed", profileHandler.GetFeed)

			places := protected.Group("/places")
			{
				places.GET("/nearby", placeHandler.SearchNearby)
				places.POST("/:place_id/checkin/initiate", checkinHandler.Initiate)
				places.POST("/:place_id/checkin/confirm", checkinHandler.Confirm)
			}

			rankings := protected.Group("/rankings")
			{
				rankings.GET("/global", rankingHandler.Global)
				rankings.GET("/friends", rankingHandler.Friends)
			}

			users := protected.Group("/users")
			{
				users.GET("/:username", userHandler.GetProfile)
				users.POST("/:user_id/block", userHandler.Block)
				users.DELETE("/:user_id/block", userHandler.Unblock)
				users.POST("/:user_id/friend-request", friendHandler.SendRequest)
				users.DELETE("/:user_id/friend", friendHandler.RemoveFriend)
			}

			friendRequests := protected.Group("/friend-requests")
			{
				friendRequests.GET("/incoming", friendHandler.GetIncoming)
				friendRequests.GET("/outgoing", friendHandler.GetOutgoing)
				friendRequests.POST("/:user_id/accept", friendHandler.AcceptRequest)
				friendRequests.POST("/:user_id/reject", friendHandler.RejectRequest)
				friendRequests.DELETE("/:user_id", friendHandler.CancelRequest)
			}

			challenges := protected.Group("/challenges")
			{
				challenges.POST("", challengeHandler.Create)
				challenges.GET("/incoming", challengeHandler.GetIncoming)
				challenges.GET("/outgoing", challengeHandler.GetOutgoing)
				challenges.GET("/:challenge_id", challengeHandler.GetByID)
				challenges.POST("/:challenge_id/accept", challengeHandler.Accept)
				challenges.POST("/:challenge_id/reject", challengeHandler.Reject)
			}
		}
	}

	return router
}

func healthHandler(env string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"service": "nomad-api",
			"env":     env,
		})
	}
}
