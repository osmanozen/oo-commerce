package persistence

import (
	"context"
	"embed"

	bbpersistence "github.com/osmanozen/oo-commerce/src/pkg/buildingblocks/persistence"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func RunMigrations(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	return bbpersistence.RunEmbeddedMigrations(ctx, pool, bbpersistence.MigrateOptions{
		ServiceName: "wishlists",
		Migrations:  migrationsFS,
		Dir:         "migrations",
	})
}
