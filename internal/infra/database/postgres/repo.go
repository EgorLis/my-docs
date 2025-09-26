package postgres

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// ---- Postgres репозиторий (pgxpool) + golang-migrate ----

type PGRepo struct {
	logger *log.Logger
	pool   *pgxpool.Pool
	schema string
}

func NewPGRepo(ctx context.Context, logger *log.Logger, dsn, schema string) (*PGRepo, error) {
	// Запускаем golang-migrate используя pgx/stdlib
	if err := runMigrations(dsn, logger); err != nil {
		return nil, fmt.Errorf("migrations: %w", err)
	}

	// Создаем pgxpool
	logger.Println("initializing pgxpool...")
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	logger.Println("pgxpool initialized")

	r := &PGRepo{pool: pool, schema: schema, logger: logger}
	return r, nil
}

func (r *PGRepo) Close() {
	r.logger.Println("closing pgxpool...")
	r.pool.Close()
	r.logger.Println("pgxpool closed")
}

// ---- Миграции через golang-migrate ----

//go:embed migrations/*.sql
var EmbeddedMigrations embed.FS

func runMigrations(dsn string, logger *log.Logger) error {
	// Открываем *sql.DB с помощью pgx stdlib. Важно: это отдельный экземпляр от pgxpool.
	sqldb, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("sql.Open pgx: %w", err)
	}
	defer sqldb.Close()

	driver, err := postgres.WithInstance(sqldb, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("postgres driver: %w", err)
	}

	src, err := iofs.New(EmbeddedMigrations, "migrations")
	if err != nil {
		return fmt.Errorf("iofs source: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return fmt.Errorf("migrate.New: %w", err)
	}
	defer m.Close()

	logger.Println("applying migrations...")
	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			logger.Println("no new migrations to apply")
			return nil
		}
		return fmt.Errorf("apply migrations: %w", err)
	}
	logger.Println("migrations applied successfully")
	return nil
}

// ---- Реализация репозитория ----

func (r *PGRepo) Ping(ctx context.Context) error {
	r.logger.Println("pinging database...")
	if err := r.pool.Ping(ctx); err != nil {
		r.logger.Printf("ping failed: %v", err)
		return err
	}
	r.logger.Println("ping successful")
	return nil
}
