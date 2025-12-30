package bot

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
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
	UserStates      map[int64]string
	StatesMu        sync.RWMutex
	SquadID         string
}

func NewBot(token string, paymentClient *payment.Client, remnawaveClient *remnawave.Client, db *gorm.DB, squadID string) (*Bot, error) {
	tgBot, err := telego.NewBot(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	return &Bot{
		Instance:        tgBot,
		PaymentClient:   paymentClient,
		RemnawaveClient: remnawaveClient,
		DB:              db,
		UserStates:      make(map[int64]string),
		SquadID:         squadID,
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
				tu.InlineKeyboardButton("👤 Личный кабинет").WithCallbackData("profile"),
				tu.InlineKeyboardButton("💰 Пополнить баланс").WithCallbackData("topup_balance"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("🚀 Купить VPN (255₽)").WithCallbackData("buy_subscription_balance"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("🤝 Партнерская программа").WithCallbackData("invite_friend"),
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
				tu.InlineKeyboardButton("🚀 VPN 30 дней - 255₽").WithCallbackData("buy_subscription_balance"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("« Назад").WithCallbackData("start_back"),
			),
		)

		_, _ = ctx.Bot().SendMessage(ctx.Context(), tu.Message(
			tu.ID(callback.From.ID),
			"📊 Тарифный план:\nVPN на 30 дней за 255 рублей.\nОплата списывается с внутреннего баланса.",
		).WithReplyMarkup(keyboard))
		_ = ctx.Bot().AnswerCallbackQuery(ctx.Context(), tu.CallbackQuery(callback.ID))
		return nil
	}, th.CallbackDataEqual("buy_vpn"))

	// Callback for buying subscription from balance
	handler.Handle(func(ctx *th.Context, update telego.Update) error {
		callback := update.CallbackQuery
		telegramID := callback.From.ID

		// Get User
		var user models.User
		if err := b.DB.Where("telegram_id = ?", telegramID).First(&user).Error; err != nil {
			_, _ = ctx.Bot().SendMessage(ctx.Context(), tu.Message(tu.ID(telegramID), "❌ Ошибка: пользователь не найден."))
			return nil
		}

		price := 255.0
		durationDays := 30

		// Check Balance
		if user.Balance < price {
			keyboard := tu.InlineKeyboard(
				tu.InlineKeyboardRow(
					tu.InlineKeyboardButton("💰 Пополнить баланс").WithCallbackData("topup_balance"),
				),
				tu.InlineKeyboardRow(
					tu.InlineKeyboardButton("« Назад").WithCallbackData("buy_vpn"),
				),
			)
			msg := fmt.Sprintf("❌ Недостаточно средств.\nВаш баланс: %.2f₽\nСтоимость: %.2f₽", user.Balance, price)
			_, _ = ctx.Bot().SendMessage(ctx.Context(), tu.Message(tu.ID(telegramID), msg).WithReplyMarkup(keyboard))
			_ = ctx.Bot().AnswerCallbackQuery(ctx.Context(), tu.CallbackQuery(callback.ID))
			return nil
		}

		// Process Purchase
		// 1. Deduct Balance
		user.Balance -= price
		if err := b.DB.Save(&user).Error; err != nil {
			log.Printf("Failed to update balance: %v", err)
			_, _ = ctx.Bot().SendMessage(ctx.Context(), tu.Message(tu.ID(telegramID), "❌ Ошибка при списании средств."))
			return nil
		}

		// 2. Activate/Extend Subscription
		var sub models.Subscription
		dbResult := b.DB.Where("user_id = ?", user.ID).First(&sub)

		var vpnLink string
		var expireDate time.Time

		if dbResult.Error == gorm.ErrRecordNotFound {
			// New Subscription
			rwUser, err := b.RemnawaveClient.CreateUser(telegramID, fmt.Sprintf("user_%d", telegramID), durationDays, b.SquadID)
			if err != nil {
				// Rollback balance (simple manual rollback)
				user.Balance += price
				b.DB.Save(&user)
				log.Printf("Failed to create Remnawave user: %v", err)
				_, _ = ctx.Bot().SendMessage(ctx.Context(), tu.Message(tu.ID(telegramID), "❌ Ошибка при активации VPN. Средства возвращены."))
				return nil
			}

			vpnLink = rwUser.SubscriptionURL
			expireDate = time.Now().Add(time.Duration(durationDays) * 24 * time.Hour)

			newSub := models.Subscription{
				UserID:          user.ID,
				RemnawaveID:     rwUser.UUID,
				SubscriptionURL: rwUser.SubscriptionURL,
				ExpirationDate:  expireDate,
				PlanType:        "standard",
			}
			b.DB.Create(&newSub)

		} else {
			// Extend Subscription
			if err := b.RemnawaveClient.ExtendSubscription(sub.RemnawaveID, durationDays); err != nil {
				// Rollback
				user.Balance += price
				b.DB.Save(&user)
				log.Printf("Failed to extend Remnawave user: %v", err)
				_, _ = ctx.Bot().SendMessage(ctx.Context(), tu.Message(tu.ID(telegramID), "❌ Ошибка при продлении VPN. Средства возвращены."))
				return nil
			}

			// Calculate new expiry
			if sub.ExpirationDate.Before(time.Now()) {
				expireDate = time.Now().Add(time.Duration(durationDays) * 24 * time.Hour)
			} else {
				expireDate = sub.ExpirationDate.Add(time.Duration(durationDays) * 24 * time.Hour)
			}

			sub.ExpirationDate = expireDate
			b.DB.Save(&sub)

			// Try get link if missing
			if sub.SubscriptionURL == "" {
				if rwUser, err := b.RemnawaveClient.GetUser(sub.RemnawaveID); err == nil {
					sub.SubscriptionURL = rwUser.SubscriptionURL
					b.DB.Save(&sub)
				}
			}
			vpnLink = sub.SubscriptionURL
		}

		// Success Message
		msg := fmt.Sprintf("✅ Подписка активирована!\n\n📅 Действует до: %s\n\n🔗 *Ссылка на VPN:*\n%s", expireDate.Format("02.01.2006"), vpnLink)
		_, _ = ctx.Bot().SendMessage(ctx.Context(), tu.Message(tu.ID(telegramID), msg).WithParseMode(telego.ModeMarkdown))
		_ = ctx.Bot().AnswerCallbackQuery(ctx.Context(), tu.CallbackQuery(callback.ID))
		return nil

	}, th.CallbackDataEqual("buy_subscription_balance"))

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

		msg := fmt.Sprintf("👤 *Личный кабинет:*\n\n🔹 ID: `%d`\n🔹 Баланс: %.2f₽\n🔹 Статус: %s\n🔹 Действует до: %s", telegramID, user.Balance, status, expiry)

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

		keyboard := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("💰 Пополнить баланс").WithCallbackData("topup_balance"),
			),
		)

		_, _ = ctx.Bot().SendMessage(ctx.Context(), tu.Message(tu.ID(telegramID), msg).WithParseMode(telego.ModeMarkdown).WithReplyMarkup(keyboard))
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

		msg := fmt.Sprintf("🤝 *Партнерская программа*\n\n"+
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
				tu.InlineKeyboardButton("👤 Личный кабинет").WithCallbackData("profile"),
				tu.InlineKeyboardButton("💰 Пополнить баланс").WithCallbackData("topup_balance"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("🚀 Купить VPN").WithCallbackData("buy_subscription_balance"),
				tu.InlineKeyboardButton("🤝 Партнерам").WithCallbackData("invite_friend"),
			),
		)

		_, _ = ctx.Bot().SendMessage(ctx.Context(), tu.Message(
			tu.ID(callback.From.ID),
			"Привет! 👋\n\nЯ помогу тебе с VPN через Remnawave.",
		).WithReplyMarkup(keyboard))
		_ = ctx.Bot().AnswerCallbackQuery(ctx.Context(), tu.CallbackQuery(callback.ID))
		return nil
	}, th.CallbackDataEqual("start_back"))

	// Callback for Top Up Balance Request
	handler.Handle(func(ctx *th.Context, update telego.Update) error {
		telegramID := update.CallbackQuery.From.ID

		b.StatesMu.Lock()
		b.UserStates[telegramID] = "WAITING_TOPUP_AMOUNT"
		b.StatesMu.Unlock()

		_, _ = ctx.Bot().SendMessage(ctx.Context(), tu.Message(tu.ID(telegramID), "💰 Введите сумму пополнения (минимум 100₽):"))
		_ = ctx.Bot().AnswerCallbackQuery(ctx.Context(), tu.CallbackQuery(update.CallbackQuery.ID))
		return nil
	}, th.CallbackDataEqual("topup_balance"))

	// Handle Text Input (for Top Up)
	handler.Handle(func(ctx *th.Context, update telego.Update) error {
		telegramID := update.Message.From.ID
		text := update.Message.Text

		b.StatesMu.RLock()
		state, ok := b.UserStates[telegramID]
		b.StatesMu.RUnlock()

		if !ok || state != "WAITING_TOPUP_AMOUNT" {
			return nil // Pass to next handler if any
		}

		// Process Amount
		amount, err := strconv.ParseFloat(text, 64)
		if err != nil || amount < 100 {
			_, _ = ctx.Bot().SendMessage(ctx.Context(), tu.Message(tu.ID(telegramID), "❌ Некорректная сумма. Введите число не меньше 100."))
			return nil
		}

		// Create Payment
		metadata := map[string]string{
			"telegram_id": strconv.FormatInt(telegramID, 10),
			"type":        "balance_topup",
		}

		paymentResp, err := b.PaymentClient.CreatePayment(fmt.Sprintf("%.2f", amount), "RUB", "Пополнение баланса", "https://t.me/your_bot_name", metadata)
		if err != nil {
			log.Printf("Failed to create topup payment: %v", err)
			_, _ = ctx.Bot().SendMessage(ctx.Context(), tu.Message(tu.ID(telegramID), "❌ Ошибка при создании платежа."))
		} else {
			_, _ = ctx.Bot().SendMessage(ctx.Context(), tu.Message(
				tu.ID(telegramID),
				fmt.Sprintf("💳 Ссылка для пополнения на %.2f₽:\n%s", amount, paymentResp.Confirmation.ConfirmationURL),
			))
		}

		// Reset State
		b.StatesMu.Lock()
		delete(b.UserStates, telegramID)
		b.StatesMu.Unlock()

		return nil
	}, th.AnyMessageWithText())

	handler.Start()
}
