package persistence

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MigrateOptions struct {
	ServiceName string
	Migrations  embed.FS
	Dir         string
}

func RunEmbeddedMigrations(ctx context.Context, pool *pgxpool.Pool, opts MigrateOptions) (int, error) {
	if opts.ServiceName == "" {
		return 0, fmt.Errorf("service name is required")
	}
	if opts.Dir == "" {
		return 0, fmt.Errorf("migrations directory is required")
	}

	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS public.schema_migrations (
			service_name text NOT NULL,
			migration_name text NOT NULL,
			applied_at timestamptz NOT NULL DEFAULT now(),
			PRIMARY KEY (service_name, migration_name)
		)
	`); err != nil {
		return 0, fmt.Errorf("ensure schema migrations table: %w", err)
	}

	entries, err := fs.ReadDir(opts.Migrations, opts.Dir)
	if err != nil {
		return 0, fmt.Errorf("read migrations directory: %w", err)
	}

	migrationNames := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".up.sql") {
			migrationNames = append(migrationNames, name)
		}
	}
	sort.Strings(migrationNames)

	applied := 0
	for _, name := range migrationNames {
		var alreadyApplied bool
		if err := pool.QueryRow(
			ctx,
			`SELECT EXISTS(
				SELECT 1
				FROM public.schema_migrations
				WHERE service_name = $1 AND migration_name = $2
			)`,
			opts.ServiceName,
			name,
		).Scan(&alreadyApplied); err != nil {
			return applied, fmt.Errorf("check migration %s state: %w", name, err)
		}
		if alreadyApplied {
			continue
		}

		content, err := opts.Migrations.ReadFile(opts.Dir + "/" + name)
		if err != nil {
			return applied, fmt.Errorf("read migration %s: %w", name, err)
		}

		tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return applied, fmt.Errorf("begin migration tx %s: %w", name, err)
		}

		if _, err := tx.Exec(ctx, string(content)); err != nil {
			_ = tx.Rollback(ctx)
			return applied, fmt.Errorf("apply migration %s: %w", name, err)
		}
		if _, err := tx.Exec(
			ctx,
			`INSERT INTO public.schema_migrations (service_name, migration_name) VALUES ($1, $2)`,
			opts.ServiceName,
			name,
		); err != nil {
			_ = tx.Rollback(ctx)
			return applied, fmt.Errorf("record migration %s: %w", name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			_ = tx.Rollback(ctx)
			return applied, fmt.Errorf("commit migration %s: %w", name, err)
		}
		applied++
	}

	return applied, nil
}
