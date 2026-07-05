package webdeps

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/tuusuario/candela-api/internal/config"
	"github.com/tuusuario/candela-api/internal/infra/storage"
)

type Dependencies struct {
	Env     string
	DBPool  *pgxpool.Pool
	Redis   *redis.Client
	Config  config.Config
	Storage storage.ObjectStorage
}
