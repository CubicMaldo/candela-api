package webdeps

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/tuusuario/candela-api/internal/config"
	"github.com/tuusuario/candela-api/internal/handler"
	"github.com/tuusuario/candela-api/internal/infra/storage"
	"github.com/tuusuario/candela-api/internal/repo"
	"github.com/tuusuario/candela-api/internal/service"
)

type Dependencies struct {
	Env     string
	DBPool  *pgxpool.Pool
	Redis   *redis.Client
	Config  config.Config
	Storage storage.ObjectStorage
}

// Handlers bundles all presentation layer HTTP handlers.
type Handlers struct {
	Auth    *handler.AuthHandler
	Place   *handler.PlaceHandler
	Checkin *handler.CheckInHandler
	Ranking *handler.RankingHandler
	User    *handler.UserHandler
	Friend  *handler.FriendHandler
	Plan    *handler.PlanHandler
	Profile *handler.ProfileHandler
	Follow  *handler.FollowHandler
	Report  *handler.ReportHandler
}

// BuildHandlers assembles the repositories, application services, and HTTP handlers.
func BuildHandlers(deps Dependencies) Handlers {
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
	return Handlers{
		Auth:    handler.NewAuthHandler(authService),
		Place:   handler.NewPlaceHandler(placeService),
		Checkin: handler.NewCheckInHandler(checkinService),
		Ranking: handler.NewRankingHandler(rankingService),
		User:    handler.NewUserHandler(userService),
		Friend:  handler.NewFriendHandler(friendService),
		Plan:    handler.NewPlanHandler(planService),
		Profile: handler.NewProfileHandler(profileService),
		Follow:  handler.NewFollowHandler(followService),
		Report:  handler.NewReportHandler(reportService),
	}
}
