package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DBUser           string
	DBPassword       string
	DBName           string
	DBHost           string
	DBPort           string
	RedisHost        string
	RedisPort        string
	RedisPassword    string
	BotToken         string
	RemnawaveURL     string
	RemnawaveKey     string
	RemnawaveSquadID string
	YookassaShopID   string
	YookassaKey      string
}

func LoadConfig() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	return &Config{
		DBUser:           getEnv("DB_USER", "postgres"),
		DBPassword:       getEnv("DB_PASSWORD", "postgres"),
		DBName:           getEnv("DB_NAME", "popovka_bot"),
		DBHost:           getEnv("DB_HOST", "localhost"),
		DBPort:           getEnv("DB_PORT", "5432"),
		RedisHost:        getEnv("REDIS_HOST", "localhost"),
		RedisPort:        getEnv("REDIS_PORT", "6379"),
		RedisPassword:    getEnv("REDIS_PASSWORD", ""),
		BotToken:         getEnv("TELEGRAM_BOT_TOKEN", ""),
		RemnawaveURL:     getEnv("REMNAWAVE_API_URL", ""),
		RemnawaveKey:     getEnv("REMNAWAVE_API_KEY", ""),
		RemnawaveSquadID: getEnv("REMNAWAVE_SQUAD_ID", ""),
		YookassaShopID:   getEnv("YOOKASSA_SHOP_ID", ""),
		YookassaKey:      getEnv("YOOKASSA_SECRET_KEY", ""),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
