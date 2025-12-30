package main

import (
	"log"
	"net/http"

	"popovka-bot/internal/bot"
	"popovka-bot/internal/config"
	"popovka-bot/internal/database"
	"popovka-bot/internal/payment"
	"popovka-bot/internal/remnawave"
)

func main() {
	// Load Configuration
	cfg := config.LoadConfig()

	// Connect to Database
	db, err := database.ConnectPostgres(cfg)
	if err != nil {
		log.Fatalf("Could not connect to database: %v", err)
	}

	// Connect to Redis
	rdb, err := database.ConnectRedis(cfg)
	if err != nil {
		log.Fatalf("Could not connect to redis: %v", err)
	}
	_ = rdb

	// Initialize Remnawave Client
	remnawaveClient := remnawave.NewClient(cfg.RemnawaveURL, cfg.RemnawaveKey)
	log.Printf("Initialized Remnawave Client with URL: %s", remnawaveClient.BaseURL)

	// Initialize Payment Client
	paymentClient := payment.NewClient(cfg.YookassaShopID, cfg.YookassaKey)

	// Initialize Bot
	tgBot, err := bot.NewBot(cfg.BotToken, paymentClient, db)
	if err != nil {
		log.Fatalf("Could not initialize bot: %v", err)
	}

	// Initialize Handler
	paymentHandler := payment.NewHandler(remnawaveClient, db, tgBot.Instance)

	// Start Webhook Server
	go func() {
		http.HandleFunc("/yookassa-webhook", paymentHandler.HandleWebhook)
		log.Println("Starting Webhook Server on :10000")
		if err := http.ListenAndServe(":10000", nil); err != nil {
			log.Fatalf("Webhook server failed: %v", err)
		}
	}()

	log.Println("Service started successfully")

	// Start Bot
	tgBot.Start()
}
