package main

import (
	"log"

	"popovka-bot/internal/config"
	"popovka-bot/internal/database"
)

func main() {
	// Load Configuration
	cfg := config.LoadConfig()

	// Connect to Database
	db, err := database.ConnectPostgres(cfg)
	if err != nil {
		log.Fatalf("Could not connect to database: %v", err)
	}
	_ = db // Suppress unused var error for now

	// Connect to Redis
	rdb, err := database.ConnectRedis(cfg)
	if err != nil {
		log.Fatalf("Could not connect to redis: %v", err)
	}
	_ = rdb // Suppress unused var error for now

	log.Println("Service started successfully")

	// Keep the application running (mock for now, usually bot.Start())
	select {}
}
