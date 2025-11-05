package main

import (
	"log"
	"os"

	"buildprize-game/internal/server"
	"buildprize-game/internal/config"
)

func main() {
	// Load configuration
	cfg := config.Load()
	srv := server.NewServer(cfg)
	log.Printf("Starting server on port %s", cfg.Port)
	if err := srv.Start(); err != nil {
		log.Fatal("Failed to start server:", err)
		os.Exit(1)
	}
}
