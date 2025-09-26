package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/EgorLis/my-docs/internal/config"
	redisx "github.com/EgorLis/my-docs/internal/infra/cache/redis"
	"github.com/EgorLis/my-docs/internal/infra/database/postgres"
	s3storage "github.com/EgorLis/my-docs/internal/infra/storage/s3"
	"github.com/EgorLis/my-docs/internal/transport/web"
)

type App struct {
	config *config.Config
	server *web.Server
	log    *log.Logger
}

func Build(ctx context.Context) (*App, error) {
	base := log.New(os.Stdout, "[app] ", log.LstdFlags)

	serverLog := log.New(base.Writer(), base.Prefix()+"[server] ", base.Flags())
	pgLog := log.New(base.Writer(), base.Prefix()+"[postgres] ", base.Flags())
	s3Log := log.New(base.Writer(), base.Prefix()+"[s3] ", base.Flags())

	cfg, err := config.LoadFromEnv()
	if err != nil {
		return nil, fmt.Errorf("failed load config: %w", err)
	}
	base.Printf("\n  configuration: %s-------------------", cfg)

	base.Println("init PostgreSQL")
	pgRepo, err := postgres.NewPGRepo(ctx, pgLog, cfg.GetDSN(), cfg.DBScheme)
	if err != nil {
		return nil, fmt.Errorf("failed init postgres: %w", err)
	}
	base.Println("PostgreSQL is initialized")

	base.Println("init S3 storage")
	s3cfg := s3storage.Config{
		Endpoint:  cfg.S3Endpoint,
		Region:    cfg.S3Region,
		Bucket:    cfg.S3Bucket,
		AccessKey: cfg.S3AccessKey,
		SecretKey: cfg.S3SecretKey,
		UseSSL:    cfg.S3UseSSL,
		PathStyle: cfg.S3PathStyle,
	}
	s3, err := s3storage.New(ctx, s3cfg)
	if err != nil {
		return nil, fmt.Errorf("failed init s3: %w", err)
	}
	_ = s3Log // при желании логируй операции, сейчас не используем

	base.Println("init Redis")
	rc := redisx.New(redisx.Config{
		Addr:     cfg.RedisAddr,
		DB:       cfg.RedisDB,
		Password: cfg.RedisPassword,
	})
	if err := rc.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed init redis: %w", err)
	}
	base.Println("Redis is initialized")

	base.Println("init Server")
	server := web.New(serverLog, cfg, pgRepo, s3, rc)
	base.Println("Server is initialized")

	base.Println("build ended")
	return &App{config: cfg, server: server, log: base}, nil
}

func (a *App) Run(ctx context.Context) error {
	a.log.Println("start application...")
	go a.server.Run()
	<-ctx.Done()
	a.log.Println("stop application...")

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	a.server.Close(stopCtx)
	return nil
}
