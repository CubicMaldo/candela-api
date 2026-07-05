package web

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tuusuario/candela-api/internal/handler"
	"github.com/tuusuario/candela-api/internal/repo"
	"github.com/tuusuario/candela-api/internal/service"
	"github.com/tuusuario/candela-api/internal/web/middleware"
	"github.com/tuusuario/candela-api/internal/webdeps"
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
	planRepo := repo.NewPlanRepository(deps.DBPool)
	feedRepo := repo.NewFeedRepository(deps.DBPool)
	followRepo := repo.NewFollowRepository(deps.DBPool)
	reportRepo := repo.NewReportRepository(deps.DBPool)

	// Services
	authService := service.NewAuthService(userRepo, deps.Config)
	checkinService := service.NewCheckInService(checkinRepo, planRepo, userRepo, deps.Storage, deps.Redis)
	rankingService := service.NewRankingService(rankingRepo, userRepo, deps.Redis)
	userService := service.NewUserService(userRepo, friendshipRepo, deps.Redis)
	friendService := service.NewFriendService(friendshipRepo, deps.Redis)
	planService := service.NewPlanService(planRepo, friendshipRepo, placeRepo)
	profileService := service.NewProfileService(userRepo, feedRepo, checkinRepo, planRepo)
	followService := service.NewFollowService(followRepo, placeRepo)
	reportService := service.NewReportService(reportRepo, userRepo, placeRepo, checkinRepo)
	placeService := service.NewPlaceService(placeRepo, followService, checkinRepo)

	// Handlers
	authHandler := handler.NewAuthHandler(authService)
	placeHandler := handler.NewPlaceHandler(placeService)
	checkinHandler := handler.NewCheckInHandler(checkinService)
	rankingHandler := handler.NewRankingHandler(rankingService)
	userHandler := handler.NewUserHandler(userService)
	friendHandler := handler.NewFriendHandler(friendService)
	planHandler := handler.NewPlanHandler(planService)
	profileHandler := handler.NewProfileHandler(profileService)
	followHandler := handler.NewFollowHandler(followService)
	reportHandler := handler.NewReportHandler(reportService)

	api := router.Group("/api")
	{
		auth := api.Group("/auth")
		auth.Use(authLimiter)
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.POST("/google", authHandler.GoogleLogin)
			auth.POST("/google/register", authHandler.GoogleRegister)
		}

		protected := api.Group("")
		protected.Use(middleware.AuthMiddleware(deps.Config.JWTSecret, deps.Redis))
		{
			protected.GET("/me", profileHandler.GetMe)
			protected.PATCH("/me/privacy", userHandler.UpdatePrivacy)
			protected.PATCH("/me/ranking", rankingHandler.UpdateOptIn)
			protected.DELETE("/me", userHandler.DeleteAccount)
			protected.GET("/me/friends", friendHandler.GetFriends)
			protected.GET("/me/check-ins", profileHandler.GetMyCheckIns)
			protected.GET("/me/plans", profileHandler.GetMyPlans)

			protected.GET("/feed", profileHandler.GetFeed)

			places := protected.Group("/places")
			{
				places.GET("/nearby", placeHandler.SearchNearby)
				places.GET("/:place_id", placeHandler.GetDetail)
				places.GET("/:place_id/feed", placeHandler.GetEventFeed)
				places.POST("/:place_id/follow", followHandler.Follow)
				places.DELETE("/:place_id/follow", followHandler.Unfollow)
				places.POST("/:place_id/checkin/initiate", checkinHandler.Initiate)
				places.POST("/:place_id/checkin/confirm", checkinHandler.Confirm)
			}

			checkins := protected.Group("/checkins")
			{
				checkins.PATCH("/:checkin_id/permanent", checkinHandler.SetPermanent)
			}

			protected.POST("/reports", reportHandler.Create)

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

			plans := protected.Group("/plans")
			{
				plans.POST("", planHandler.Create)
				plans.GET("/discover", planHandler.Discover)
				plans.GET("/incoming", planHandler.GetIncoming)
				plans.GET("/outgoing", planHandler.GetOutgoing)
				plans.GET("/:plan_id", planHandler.GetByID)
				plans.POST("/:plan_id/accept", planHandler.Accept)
				plans.POST("/:plan_id/decline", planHandler.Decline)
				plans.POST("/:plan_id/join", planHandler.Join)
				plans.DELETE("/:plan_id", planHandler.Cancel)
			}
		}
	}

	return router
}

func healthHandler(env string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"service": "candela-api",
			"env":     env,
		})
	}
}
