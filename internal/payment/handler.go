package payment

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"popovka-bot/internal/models"
	"popovka-bot/internal/remnawave"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
	"gorm.io/gorm"
)

type Handler struct {
	RemnawaveClient *remnawave.Client
	DB              *gorm.DB
	Bot             *telego.Bot
}

func NewHandler(remnawaveClient *remnawave.Client, db *gorm.DB, bot *telego.Bot) *Handler {
	return &Handler{
		RemnawaveClient: remnawaveClient,
		DB:              db,
		Bot:             bot,
	}
}

func (h *Handler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var notification WebhookNotification
	if err := json.NewDecoder(r.Body).Decode(&notification); err != nil {
		log.Printf("Failed to decode webhook: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if notification.Event != "payment.succeeded" {
		log.Printf("Ignored event: %s", notification.Event)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Process successful payment
	if err := h.processSuccess(notification.Object); err != nil {
		log.Printf("Failed to process payment success: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) processSuccess(obj WebhookObject) error {
	log.Printf("Processing payment success: %s", obj.ID)

	telegramIDStr, ok := obj.Metadata["telegram_id"]
	if !ok {
		return fmt.Errorf("metadata missing telegram_id")
	}

	telegramID, err := strconv.ParseInt(telegramIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid telegram_id: %w", err)
	}

	durationStr, ok := obj.Metadata["duration"]
	if !ok {
		// Default duration if not specified
		durationStr = "30d"
	}

	// 1. Find or Create User in DB
	var user models.User
	if err := h.DB.FirstOrCreate(&user, models.User{TelegramID: telegramID}).Error; err != nil {
		return fmt.Errorf("failed to find/create user: %w", err)
	}

	var rwID string

	// 2. Check if subscription exists
	var sub models.Subscription
	result := h.DB.Where("user_id = ?", user.ID).First(&sub)

	if result.Error == gorm.ErrRecordNotFound {
		// New Subscription -> Create User in Remnawave
		log.Printf("Creating new Remnawave user for TelegramID: %d", telegramID)
		rwUser, err := h.RemnawaveClient.CreateUser(telegramID, fmt.Sprintf("user_%d", telegramID))
		if err != nil {
			return fmt.Errorf("remnawave create user error: %w", err)
		}

		rwID = rwUser.ID

		// Create Subscription record
		newSub := models.Subscription{
			UserID:      user.ID,
			RemnawaveID: rwUser.ID,
			PlanType:    "standard", // Default plan
		}
		if err := h.DB.Create(&newSub).Error; err != nil {
			return fmt.Errorf("failed to save subscription: %w", err)
		}

	} else if result.Error == nil {
		rwID = sub.RemnawaveID
		// Existing Subscription -> Extend
		log.Printf("Extending subscription for RemnawaveID: %s", sub.RemnawaveID)
		if err := h.RemnawaveClient.ExtendSubscription(sub.RemnawaveID, durationStr); err != nil {
			return fmt.Errorf("remnawave extend error: %w", err)
		}
	} else {
		return fmt.Errorf("db error checking subscription: %w", result.Error)
	}

	// 3. Record Payment
	amountVal, _ := strconv.ParseFloat(obj.Amount.Value, 64)
	payment := models.Payment{
		UserID:     user.ID,
		Amount:     amountVal,
		Status:     "succeeded",
		YooKassaID: obj.ID,
	}
	if err := h.DB.Create(&payment).Error; err != nil {
		log.Printf("Failed to record payment: %v", err)
	}

	// 4. Notify User
	configLink, err := h.RemnawaveClient.GetConfig(rwID)
	if err != nil {
		log.Printf("Failed to get config link for user %d: %v", telegramID, err)
		_, _ = h.Bot.SendMessage(context.Background(), tu.Message(tu.ID(telegramID), "✅ Оплата прошла успешно! Но возникла проблема при получении ссылки на конфиг. Напишите в поддержку."))
		return nil // Still success for YooKassa
	}

	_, _ = h.Bot.SendMessage(context.Background(), tu.Message(
		tu.ID(telegramID),
		fmt.Sprintf("✅ Оплата прошла успешно!\n\nТвоя ссылка на VPN:\n`%s`\n\nПриятного пользования!", configLink),
	))

	return nil
}

// rwID helper
func (h *Handler) getRWID(userID uint) (string, error) {
	var sub models.Subscription
	if err := h.DB.Where("user_id = ?", userID).First(&sub).Error; err != nil {
		return "", err
	}
	return sub.RemnawaveID, nil
}
