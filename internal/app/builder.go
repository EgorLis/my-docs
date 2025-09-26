package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/EgorLis/my-docs/internal/config"
	"github.com/EgorLis/my-docs/internal/infra/database/postgres"
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

	base.Println("init Server")
	server := web.New(serverLog, cfg, pgRepo)
	base.Println("Server is initialized")

	base.Println("build ended")

	return &App{
		config: cfg,
		server: server,
		log:    base,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	a.log.Println("start application...")

	go a.server.Run()

	<-ctx.Done()
	a.log.Println("stop application...")

	// graceful stop

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	a.server.Close(stopCtx)

	return nil
}
