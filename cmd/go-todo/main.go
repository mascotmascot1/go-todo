package main

import (
	"log"
	"os"

	"github.com/mascotmascot1/go-todo/internal/config"
	"github.com/mascotmascot1/go-todo/internal/db"
	"github.com/mascotmascot1/go-todo/internal/server"

	_ "modernc.org/sqlite"
)

// Main is the entry point of the program. It sets up the programme's parameters,
// initialises the database, sets up and runs the server.
func main() {
	logger := log.New(os.Stdout, "[GO-TODO] ", log.LstdFlags)

	// Setting up of the programme's parameters.
	cfg, err := config.New()
	if err != nil {
		logger.Println(err)
		return
	}

	// Initialising the database.
	if err := db.Init(cfg.Server.DBFile); err != nil {
		logger.Println(err)
		return
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Println(err)
		}
	}()

	srv := server.New(cfg, logger)
	logger.Printf("Starting server on %s\n", srv.HTTP.Addr)
	if err := srv.Run(); err != nil {
		logger.Println(err)
	}
}
