package worker

import (
	"context"
	"fmt"
	"log"
	"time"

	"popovka-bot/internal/models"
	"popovka-bot/internal/remnawave"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Checker struct {
	DB        *gorm.DB
	Redis     *redis.Client
	Remnawave *remnawave.Client
	Bot       *telego.Bot
}

func NewChecker(db *gorm.DB, rdb *redis.Client, rm *remnawave.Client, bot *telego.Bot) *Checker {
	return &Checker{
		DB:        db,
		Redis:     rdb,
		Remnawave: rm,
		Bot:       bot,
	}
}

func (c *Checker) Start() {
	ticker := time.NewTicker(1 * time.Hour)
	log.Println("Background subscription worker started")

	// Run once at start
	c.checkSubscriptions()

	for range ticker.C {
		c.checkSubscriptions()
	}
}

func (c *Checker) checkSubscriptions() {
	ctx := context.Background()
	now := time.Now()

	log.Println("Running subscription check cycle...")

	// 1. Notify 24h before expiry
	// Expiring in [23, 25] hours
	start := now.Add(23 * time.Hour)
	end := now.Add(25 * time.Hour)

	var expiringSoon []models.Subscription
	if err := c.DB.Preload("User").Where("expiration_date BETWEEN ? AND ?", start, end).Find(&expiringSoon).Error; err != nil {
		log.Printf("Error querying expiring subscriptions: %v", err)
	}

	for _, sub := range expiringSoon {
		key := fmt.Sprintf("notified_24h_%d", sub.UserID)
		exists, _ := c.Redis.Exists(ctx, key).Result()
		if exists == 0 {
			_, err := c.Bot.SendMessage(ctx, tu.Message(
				tu.ID(sub.User.TelegramID),
				"⚠️ Ваша подписка истекает через сутки! Пожалуйста, продлите её, чтобы не потерять доступ.",
			))
			if err == nil {
				c.Redis.Set(ctx, key, "true", 48*time.Hour)
				log.Printf("Sent 24h notification to user %d", sub.User.TelegramID)
			} else {
				log.Printf("Failed to send 24h notification to %d: %v", sub.User.TelegramID, err)
			}
		}
	}

	// 2. Handle expired subscriptions
	var expired []models.Subscription
	if err := c.DB.Preload("User").Where("expiration_date < ? AND remnawave_id != ''", now).Find(&expired).Error; err != nil {
		log.Printf("Error querying expired subscriptions: %v", err)
	}

	for _, sub := range expired {
		if sub.User.Status != "expired" {
			log.Printf("Blocking user %d due to expired subscription (expire date: %s)", sub.User.TelegramID, sub.ExpirationDate)

			err := c.Remnawave.DisableUser(sub.RemnawaveID)
			if err != nil {
				log.Printf("Failed to disable user %s in Remnawave: %v", sub.RemnawaveID, err)
				continue
			}

			// Update user status
			if err := c.DB.Model(&sub.User).Update("status", "expired").Error; err != nil {
				log.Printf("Failed to update user status in DB for %d: %v", sub.User.TelegramID, err)
			}

			_, err = c.Bot.SendMessage(ctx, tu.Message(
				tu.ID(sub.User.TelegramID),
				"❌ Ваша подписка истекла. Доступ к VPN заблокирован. Продлите подписку в меню 'Купить VPN'.",
			))
			if err != nil {
				log.Printf("Failed to send expiration notification to %d: %v", sub.User.TelegramID, err)
			}
		}
	}
}
