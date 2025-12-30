package bot

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"popovka-bot/internal/models"
	"popovka-bot/internal/payment"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
	"gorm.io/gorm"
)

type Bot struct {
	Instance      *telego.Bot
	PaymentClient *payment.Client
	DB            *gorm.DB
}

func NewBot(token string, paymentClient *payment.Client, db *gorm.DB) (*Bot, error) {
	tgBot, err := telego.NewBot(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	return &Bot{
		Instance:      tgBot,
		PaymentClient: paymentClient,
		DB:            db,
	}, nil
}

func (b *Bot) Start() {
	// Correct signature: context, params, options
	updates, _ := b.Instance.UpdatesViaLongPolling(context.Background(), nil)

	handler, _ := th.NewBotHandler(b.Instance, updates)

	// /start command
	handler.Handle(func(ctx *th.Context, update telego.Update) error {
		message := update.Message
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
