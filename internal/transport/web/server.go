package web

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/EgorLis/my-docs/internal/config"
	"github.com/EgorLis/my-docs/internal/domain"
)

type Server struct {
	logger *log.Logger
	server *http.Server
	cfg    *config.Config

	repos Repos
	auth  AuthDeps
	store domain.BlobStorage
	cache domain.Cache
}

func New(logger *log.Logger,
	cfg *config.Config,
	db Repos,
	auth AuthDeps,
	bs domain.BlobStorage,
	cache domain.Cache,
) *Server {
	server := &Server{
		cfg:    cfg,
		logger: logger,
		repos:  db,
		auth:   auth,
		store:  bs,
		cache:  cache,
	}

	http := &http.Server{
		Addr:              cfg.AppPort,
		Handler:           newRouter(server),
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		MaxHeaderBytes:    1 << 20,
		ReadHeaderTimeout: 2 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	server.server = http

	return server
}

func (ws *Server) Run() {
	ws.logger.Printf("started on %s", ws.server.Addr)
	if err := ws.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		ws.logger.Fatalf("error: %v", err)
	}
}

func (ws *Server) Close(ctx context.Context) {
	if err := ws.server.Shutdown(ctx); err != nil {
		ws.logger.Printf("forced to shutdown: %v", err)
	}
	ws.logger.Println("exited gracefully")
}
