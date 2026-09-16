package web

import (
	"os"
	"strings"
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

	// Prevent IP spoofing: do not blindly trust arbitrary X-Forwarded-For headers.
	// Only trust proxies configured via TRUSTED_PROXIES; otherwise rely strictly on RemoteAddr.
	if trustedProxies := os.Getenv("TRUSTED_PROXIES"); trustedProxies != "" {
		_ = router.SetTrustedProxies(strings.Split(trustedProxies, ","))
	} else {
		_ = router.SetTrustedProxies(nil)
	}

	// Protect against JSON / payload bombs by limiting request bodies to 1MB
	router.Use(middleware.MaxBodySize(1 << 20))

	router.GET("/health", healthHandler(deps.Env))

	// Credential endpoints are brute-force targets, so they stay tight.
	authLimiter := middleware.RateLimiter(deps.Redis, 10, time.Minute)

	// Session maintenance is not a credential guess: refresh carries a token
	// the client already holds. It also shares the caller's IP with everyone
	// behind the same NAT — a cafe or campus wifi — so the strict limit would
	// return 429 to legitimate refreshes and, because a failed refresh ends the
	// session client-side, log those users out.
	sessionLimiter := middleware.RateLimiter(deps.Redis, 120, time.Minute)

	// Repositories
	userRepo := repo.NewUserRepository(deps.DBPool)
	placeRepo := repo.NewPlaceRepository(deps.DBPool)
	checkinRepo := repo.NewCheckInRepository(deps.DBPool, deps.Config.LocalTimezone)
	rankingRepo := repo.NewRankingRepository(deps.DBPool)
	friendshipRepo := repo.NewFriendshipRepository(deps.DBPool)
	planRepo := repo.NewPlanRepository(deps.DBPool)
	feedRepo := repo.NewFeedRepository(deps.DBPool)
	followRepo := repo.NewFollowRepository(deps.DBPool)
	reportRepo := repo.NewReportRepository(deps.DBPool)
	refreshRepo := repo.NewRefreshTokenRepository(deps.DBPool)

	// Services
	authService := service.NewAuthService(userRepo, refreshRepo, deps.Config)
	checkinService := service.NewCheckInService(checkinRepo, planRepo, deps.Storage, deps.Redis)
	rankingService := service.NewRankingService(rankingRepo, userRepo, deps.Redis)
	userService := service.NewUserService(userRepo, friendshipRepo, refreshRepo, deps.Storage, deps.Redis)
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

		// Refresh and logout are unauthenticated on purpose: the caller's
		// access token has usually expired by the time they are needed. The
		// refresh token itself is the credential.
		session := api.Group("/auth")
		session.Use(sessionLimiter)
		{
			session.POST("/refresh", authHandler.Refresh)
			session.POST("/logout", authHandler.Logout)
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
				// Search lives at the collection root (GET /api/users?q=) so it
				// can't collide with the :username route below.
				users.GET("", userHandler.Search)
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
