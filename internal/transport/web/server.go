package web

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/EgorLis/my-docs/internal/config"
	"github.com/EgorLis/my-docs/internal/transport/web/v1/blob"
	"github.com/EgorLis/my-docs/internal/transport/web/v1/health"
)

type Server struct {
	log    *log.Logger
	server *http.Server
	cfg    *config.Config
}

func New(logger *log.Logger, cfg *config.Config, db health.Pinger, bs BlobStorage, cache Cache) *Server {
	healthLog := log.New(logger.Writer(), logger.Prefix()+"[health] ", logger.Flags())
	blobLog := log.New(logger.Writer(), logger.Prefix()+"[blob] ", logger.Flags())

	healthHandler := &health.Handler{DB: db, Cache: cache, Log: healthLog}
	blobHandler := &blob.Handler{Log: blobLog, Storage: bs}

	srv := &http.Server{
		Addr:              cfg.AppPort,
		Handler:           newRouter(healthHandler, blobHandler, logger),
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		MaxHeaderBytes:    1 << 20,
		ReadHeaderTimeout: 2 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return &Server{server: srv, cfg: cfg, log: logger}
}

func (ws *Server) Run() {
	ws.log.Printf("started on %s", ws.server.Addr)
	if err := ws.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		ws.log.Fatalf("error: %v", err)
	}
}

func (ws *Server) Close(ctx context.Context) {
	if err := ws.server.Shutdown(ctx); err != nil {
		ws.log.Printf("forced to shutdown: %v", err)
	}
	ws.log.Println("exited gracefully")
}
