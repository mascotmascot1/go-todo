package server

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/mascotmascot1/go-todo/internal/api"
	"github.com/mascotmascot1/go-todo/internal/config"

	"github.com/go-chi/chi/v5"
)

type server struct {
	HTTP   *http.Server
	logger *log.Logger
}

// New returns a new server instance with the given configuration and logger.
// It sets up a Chi router with the handlers for signin, nextdate, tasks, task, update, delete and task done endpoints.
// It also sets up a file server to serve static files from the web directory.
// The server is configured to listen on the address <host>:<port>, with the given timeouts.
func New(cfg *config.Config, logger *log.Logger) *server {
	r := chi.NewRouter()

	h := api.NewHandlers(&cfg.Limits, &cfg.Auth, logger)
	api.Init(r, h)

	fileServer := http.FileServer(http.Dir(cfg.Server.WebDir))
	r.Handle("/*", fileServer)

	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		ErrorLog:     logger,
		Handler:      r,
		ReadTimeout:  time.Second * 5,
		WriteTimeout: time.Second * 10,
		IdleTimeout:  time.Second * 15,
	}

	return &server{
		HTTP:   srv,
		logger: logger,
	}
}

// Run starts the server and listens on the configured address.
// It returns an error if the server failed to start, otherwise it returns nil.
func (s *server) Run() error {
	if err := s.HTTP.ListenAndServe(); err != nil {
		return fmt.Errorf("error launching server: %w", err)
	}

	return nil
}
