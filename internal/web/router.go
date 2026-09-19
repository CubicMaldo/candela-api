package web

import (
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

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

	rateLimitAuth := deps.Config.RateLimitAuth
	if rateLimitAuth <= 0 {
		rateLimitAuth = 10
	}
	rateLimitSession := deps.Config.RateLimitSession
	if rateLimitSession <= 0 {
		rateLimitSession = 120
	}

	// Credential endpoints are brute-force targets, so they stay tight.
	authLimiter := middleware.RateLimiter(deps.Redis, rateLimitAuth, time.Minute)

	// Session maintenance is not a credential guess: refresh carries a token
	// the client already holds. It also shares the caller's IP with everyone
	// behind the same NAT — a cafe or campus wifi — so the strict limit would
	// return 429 to legitimate refreshes and, because a failed refresh ends the
	// session client-side, log those users out.
	sessionLimiter := middleware.RateLimiter(deps.Redis, rateLimitSession, time.Minute)

	// Assemble handlers via webdeps
	h := webdeps.BuildHandlers(deps)

	api := router.Group("/api")
	{
		auth := api.Group("/auth")
		auth.Use(authLimiter)
		{
			auth.POST("/register", h.Auth.Register)
			auth.POST("/login", h.Auth.Login)
			auth.POST("/google", h.Auth.GoogleLogin)
			auth.POST("/google/register", h.Auth.GoogleRegister)
		}

		// Refresh and logout are unauthenticated on purpose: the caller's
		// access token has usually expired by the time they are needed. The
		// refresh token itself is the credential.
		session := api.Group("/auth")
		session.Use(sessionLimiter)
		{
			session.POST("/refresh", h.Auth.Refresh)
			session.POST("/logout", h.Auth.Logout)
		}

		protected := api.Group("")
		protected.Use(middleware.AuthMiddleware(deps.Config.JWTSecret, deps.Redis))
		{
			protected.GET("/me", h.Profile.GetMe)
			protected.PATCH("/me/privacy", h.User.UpdatePrivacy)
			protected.PATCH("/me/ranking", h.Ranking.UpdateOptIn)
			protected.DELETE("/me", h.User.DeleteAccount)
			protected.GET("/me/friends", h.Friend.GetFriends)
			protected.GET("/me/check-ins", h.Profile.GetMyCheckIns)
			protected.GET("/me/plans", h.Profile.GetMyPlans)

			protected.GET("/feed", h.Profile.GetFeed)

			places := protected.Group("/places")
			{
				places.GET("/nearby", h.Place.SearchNearby)
				places.GET("/:place_id", h.Place.GetDetail)
				places.GET("/:place_id/feed", h.Place.GetEventFeed)
				places.POST("/:place_id/follow", h.Follow.Follow)
				places.DELETE("/:place_id/follow", h.Follow.Unfollow)
				places.POST("/:place_id/checkin/initiate", h.Checkin.Initiate)
				places.POST("/:place_id/checkin/confirm", h.Checkin.Confirm)
			}

			checkins := protected.Group("/checkins")
			{
				checkins.PATCH("/:checkin_id/permanent", h.Checkin.SetPermanent)
			}

			protected.POST("/reports", h.Report.Create)

			rankings := protected.Group("/rankings")
			{
				rankings.GET("/global", h.Ranking.Global)
				rankings.GET("/friends", h.Ranking.Friends)
			}

			users := protected.Group("/users")
			{
				// Search lives at the collection root (GET /api/users?q=) so it
				// can't collide with the :username route below.
				users.GET("", h.User.Search)
				users.GET("/:username", h.User.GetProfile)
				users.POST("/:user_id/block", h.User.Block)
				users.DELETE("/:user_id/block", h.User.Unblock)
				users.POST("/:user_id/friend-request", h.Friend.SendRequest)
				users.DELETE("/:user_id/friend", h.Friend.RemoveFriend)
			}

			friendRequests := protected.Group("/friend-requests")
			{
				friendRequests.GET("/incoming", h.Friend.GetIncoming)
				friendRequests.GET("/outgoing", h.Friend.GetOutgoing)
				friendRequests.POST("/:user_id/accept", h.Friend.AcceptRequest)
				friendRequests.POST("/:user_id/reject", h.Friend.RejectRequest)
				friendRequests.DELETE("/:user_id", h.Friend.CancelRequest)
			}

			plans := protected.Group("/plans")
			{
				plans.POST("", h.Plan.Create)
				plans.GET("/discover", h.Plan.Discover)
				plans.GET("/incoming", h.Plan.GetIncoming)
				plans.GET("/outgoing", h.Plan.GetOutgoing)
				plans.GET("/:plan_id", h.Plan.GetByID)
				plans.POST("/:plan_id/accept", h.Plan.Accept)
				plans.POST("/:plan_id/decline", h.Plan.Decline)
				plans.POST("/:plan_id/join", h.Plan.Join)
				plans.DELETE("/:plan_id", h.Plan.Cancel)
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
