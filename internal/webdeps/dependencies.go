package webdeps

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tuusuario/nomad-api/internal/config"
)

type Dependencies struct {
	Env    string
	DBPool *pgxpool.Pool
	Config config.Config
}