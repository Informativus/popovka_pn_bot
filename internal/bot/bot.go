package bot

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"popovka-bot/internal/models"
	"popovka-bot/internal/payment"
	"popovka-bot/internal/remnawave"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
	"gorm.io/gorm"
)

type Bot struct {
	Instance        *telego.Bot
	PaymentClient   *payment.Client
	RemnawaveClient *remnawave.Client
	DB              *gorm.DB
}

func NewBot(token string, paymentClient *payment.Client, remnawaveClient *remnawave.Client, db *gorm.DB) (*Bot, error) {
	tgBot, err := telego.NewBot(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	return &Bot{
		Instance:        tgBot,
		PaymentClient:   paymentClient,
		RemnawaveClient: remnawaveClient,
		DB:              db,
	}, nil
}

func (b *Bot) Start() {
	// Correct signature: context, params, options
	updates, _ := b.Instance.UpdatesViaLongPolling(context.Background(), nil)

	handler, _ := th.NewBotHandler(b.Instance, updates)

	// /start command
	handler.Handle(func(ctx *th.Context, update telego.Update) error {
		message := update.Message
		telegramID := message.From.ID

		// Parse arguments manually
		args := ""
		if parts := strings.Split(message.Text, " "); len(parts) > 1 {
			args = parts[1]
		}

		// Find or Create User
		var user models.User
		if err := b.DB.FirstOrCreate(&user, models.User{TelegramID: telegramID}).Error; err != nil {
			log.Printf("Failed to get/create user: %v", err)
		}

		// Generate Referral Code if missing
		if user.ReferralCode == "" {
			user.ReferralCode = fmt.Sprintf("ref_%d", telegramID)
			user.Username = message.From.Username // Update username too
			if err := b.DB.Save(&user).Error; err != nil {
				log.Printf("Failed to update user referral code: %v", err)
			}
		}

		// Process Referral (only if new user or no referrer set)
		if args != "" && user.ReferrerID == nil && args != user.ReferralCode {
			var referrer models.User
			if err := b.DB.Where("referral_code = ?", args).First(&referrer).Error; err == nil {
				// Referrer found
				user.ReferrerID = &referrer.ID
				if err := b.DB.Save(&user).Error; err != nil {
					log.Printf("Failed to save referrer: %v", err)
				}
				log.Printf("User %d invited by %d", telegramID, referrer.TelegramID)
			}
		}

		keyboard := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("💳 Купить VPN").WithCallbackData("buy_vpn"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("👤 Мой профиль").WithCallbackData("profile"),
				tu.InlineKeyboardButton("🤝 Пригласить друга").WithCallbackData("invite_friend"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("📖 Инструкция").WithCallbackData("instruction"),
			),
		)

		_, _ = ctx.Bot().SendMessage(ctx.Context(), tu.Message(
			tu.ID(message.Chat.ID),
			fmt.Sprintf("Привет, %s! 👋\n\nЯ помогу тебе с VPN через Remnawave.", message.From.FirstName),
		).WithReplyMarkup(keyboard))
		return nil
	}, th.CommandEqual("start"))

	// Callback for "Buy VPN" - Selection of tariffs
	handler.Handle(func(ctx *th.Context, update telego.Update) error {
		callback := update.CallbackQuery
		keyboard := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("1 месяц - 299₽").WithCallbackData("buy_1m"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("3 месяца - 799₽").WithCallbackData("buy_3m"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("« Назад").WithCallbackData("start_back"),
			),
		)

		_, _ = ctx.Bot().SendMessage(ctx.Context(), tu.Message(
			tu.ID(callback.From.ID),
			"📊 Выберите подходящий тарифный план:",
		).WithReplyMarkup(keyboard))
		_ = ctx.Bot().AnswerCallbackQuery(ctx.Context(), tu.CallbackQuery(callback.ID))
		return nil
	}, th.CallbackDataEqual("buy_vpn"))

	// Callback for buying 1 month VPN
	handler.Handle(func(ctx *th.Context, update telego.Update) error {
		callback := update.CallbackQuery
		telegramID := callback.From.ID

		metadata := map[string]string{
			"telegram_id": strconv.FormatInt(telegramID, 10),
			"duration":    "30d",
		}

		paymentResp, err := b.PaymentClient.CreatePayment("299.00", "RUB", "VPN Subscription - 1 month", "https://t.me/your_bot_name", metadata)
		if err != nil {
			log.Printf("Failed to create payment: %v", err)
			_, _ = ctx.Bot().SendMessage(ctx.Context(), tu.Message(tu.ID(telegramID), "❌ Ошибка при создании платежа. Попробуйте позже."))
			return nil
		}

		_, _ = ctx.Bot().SendMessage(ctx.Context(), tu.Message(
			tu.ID(telegramID),
			fmt.Sprintf("💳 Оплата создана! Ссылка для оплаты:\n%s", paymentResp.Confirmation.ConfirmationURL),
		))

		_ = ctx.Bot().AnswerCallbackQuery(ctx.Context(), tu.CallbackQuery(callback.ID))
		return nil
	}, th.CallbackDataEqual("buy_1m"))

	// Callback for buying 3 months VPN
	handler.Handle(func(ctx *th.Context, update telego.Update) error {
		callback := update.CallbackQuery
		telegramID := callback.From.ID

		metadata := map[string]string{
			"telegram_id": strconv.FormatInt(telegramID, 10),
			"duration":    "90d",
		}

		paymentResp, err := b.PaymentClient.CreatePayment("799.00", "RUB", "VPN Subscription - 3 months", "https://t.me/your_bot_name", metadata)
		if err != nil {
			log.Printf("Failed to create payment: %v", err)
			_, _ = ctx.Bot().SendMessage(ctx.Context(), tu.Message(tu.ID(telegramID), "❌ Ошибка при создании платежа. Попробуйте позже."))
			return nil
		}

		_, _ = ctx.Bot().SendMessage(ctx.Context(), tu.Message(
			tu.ID(telegramID),
			fmt.Sprintf("💳 Оплата создана! Ссылка для оплаты:\n%s", paymentResp.Confirmation.ConfirmationURL),
		))

		_ = ctx.Bot().AnswerCallbackQuery(ctx.Context(), tu.CallbackQuery(callback.ID))
		return nil
	}, th.CallbackDataEqual("buy_3m"))

	// Callback for Profile
	handler.Handle(func(ctx *th.Context, update telego.Update) error {
		callback := update.CallbackQuery
		telegramID := callback.From.ID

		var user models.User
		if err := b.DB.Where("telegram_id = ?", telegramID).First(&user).Error; err != nil {
			_, _ = ctx.Bot().SendMessage(ctx.Context(), tu.Message(tu.ID(telegramID), "👤 Профиль не найден. Сначала купите подписку."))
			_ = ctx.Bot().AnswerCallbackQuery(ctx.Context(), tu.CallbackQuery(callback.ID))
			return nil
		}

		var sub models.Subscription
		err := b.DB.Where("user_id = ?", user.ID).First(&sub).Error

		status := "❌ Нет подписки"
		expiry := "N/A"

		if err == nil {
			status = "✅ Активна"
			expiry = sub.ExpirationDate.Format("02.01.2006")
			if sub.ExpirationDate.Before(time.Now()) {
				status = "⚠️ Истекла"
			}
		}

		msg := fmt.Sprintf("👤 *Твой профиль:*\n\n🔹 ID: `%d`\n🔹 Статус: %s\n🔹 Действует до: %s", telegramID, status, expiry)

		// Add VPN link if subscription is active
		if err == nil {
			if sub.SubscriptionURL == "" && sub.RemnawaveID != "" {
				// URL missing in DB (legacy record), try to fetch it
				rwUser, err := b.RemnawaveClient.GetUser(sub.RemnawaveID)
				if err != nil {
					log.Printf("Failed to fetch user %s from Remnawave: %v", sub.RemnawaveID, err)
				} else {
					// Update DB
					sub.SubscriptionURL = rwUser.SubscriptionURL
					if err := b.DB.Save(&sub).Error; err != nil {
						log.Printf("Failed to update subscription URL in DB: %v", err)
					}
				}
			}

			if sub.SubscriptionURL != "" {
				msg += fmt.Sprintf("\n\n🔗 *Твоя ссылка на VPN:*\n%s", sub.SubscriptionURL)
			}
		}

		_, _ = ctx.Bot().SendMessage(ctx.Context(), tu.Message(tu.ID(telegramID), msg).WithParseMode(telego.ModeMarkdown))
		_ = ctx.Bot().AnswerCallbackQuery(ctx.Context(), tu.CallbackQuery(callback.ID))
		return nil
	}, th.CallbackDataEqual("profile"))

	// Callback for Instruction
	handler.Handle(func(ctx *th.Context, update telego.Update) error {
		callback := update.CallbackQuery
		telegramID := callback.From.ID

		msg := "📖 *Как пользоваться VPN:*\n\n" +
			"1. Купите подписку через кнопку 'Купить VPN'.\n" +
			"2. После оплаты вы получите ссылку.\n" +
			"3. Скачайте приложение (V2RayNG для Android, v2BOX для iOS).\n" +
			"4. Импортируйте ссылку в приложение.\n" +
			"5. Нажмите 'Подключиться'!"

		_, _ = ctx.Bot().SendMessage(ctx.Context(), tu.Message(tu.ID(telegramID), msg).WithParseMode(telego.ModeMarkdown))
		_ = ctx.Bot().AnswerCallbackQuery(ctx.Context(), tu.CallbackQuery(callback.ID))
		return nil
	}, th.CallbackDataEqual("instruction"))

	// Callback for Invite Friend
	handler.Handle(func(ctx *th.Context, update telego.Update) error {
		callback := update.CallbackQuery
		telegramID := callback.From.ID

		var user models.User
		if err := b.DB.Where("telegram_id = ?", telegramID).First(&user).Error; err != nil {
			_, _ = ctx.Bot().SendMessage(ctx.Context(), tu.Message(tu.ID(telegramID), "❌ Ошибка: пользователь не найден."))
			_ = ctx.Bot().AnswerCallbackQuery(ctx.Context(), tu.CallbackQuery(callback.ID))
			return nil
		}

		// Ensure referral code exists
		if user.ReferralCode == "" {
			user.ReferralCode = fmt.Sprintf("ref_%d", telegramID)
			b.DB.Save(&user)
		}

		// Get Stats
		var invitedCount int64
		b.DB.Model(&models.User{}).Where("referrer_id = ?", user.ID).Count(&invitedCount)

		var totalEarned float64
		b.DB.Model(&models.ReferralTransaction{}).Where("referrer_id = ?", user.ID).Select("COALESCE(SUM(amount), 0)").Scan(&totalEarned)

		botUsername := "popovka_bot" // TODO: Get from config or context
		if info, err := b.Instance.GetMe(ctx.Context()); err == nil {
			botUsername = info.Username
		}
		refLink := fmt.Sprintf("https://t.me/%s?start=%s", botUsername, user.ReferralCode)

		msg := fmt.Sprintf("🤝 *Реферальная программа*\n\n"+
			"Приглашай друзей и получай бонусы!\n\n"+
			"👥 Приглашено: %d\n"+
			"💰 Заработано: %.2f₽\n\n"+
			"🔗 *Твоя ссылка:*\n`%s`", invitedCount, totalEarned, refLink)

		// Keyboard with Back button
		keyboard := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("« Назад").WithCallbackData("start_back"),
			),
		)

		_, _ = ctx.Bot().SendMessage(ctx.Context(), tu.Message(tu.ID(telegramID), msg).WithParseMode(telego.ModeMarkdown).WithReplyMarkup(keyboard))
		_ = ctx.Bot().AnswerCallbackQuery(ctx.Context(), tu.CallbackQuery(callback.ID))
		return nil
	}, th.CallbackDataEqual("invite_friend"))

	// Callback for Back to Start
	handler.Handle(func(ctx *th.Context, update telego.Update) error {
		callback := update.CallbackQuery
		keyboard := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("💳 Купить VPN").WithCallbackData("buy_vpn"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("👤 Мой профиль").WithCallbackData("profile"),
				tu.InlineKeyboardButton("📖 Инструкция").WithCallbackData("instruction"),
			),
		)

		_, _ = ctx.Bot().SendMessage(ctx.Context(), tu.Message(
			tu.ID(callback.From.ID),
			"Привет! 👋\n\nЯ помогу тебе с VPN через Remnawave.",
		).WithReplyMarkup(keyboard))
		_ = ctx.Bot().AnswerCallbackQuery(ctx.Context(), tu.CallbackQuery(callback.ID))
		return nil
	}, th.CallbackDataEqual("start_back"))

	handler.Start()
}
