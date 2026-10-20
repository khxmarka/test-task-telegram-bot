package telegram

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"go.uber.org/zap"

	"github.com/testTelegramApp/expenses-bot/internal/agent"
	"github.com/testTelegramApp/expenses-bot/internal/domain"
	"github.com/testTelegramApp/expenses-bot/internal/service"
)

type Handler struct {
	bot    *bot.Bot
	users  *service.UserService
	agent  *agent.Agent
	log    *zap.Logger
	path   string
	cancel context.CancelFunc
	mode   string // "webhook" | "polling"
}

// New creates a Telegram bot. Delivery:
//   - webhookURL != "" → webhook mode (TZ default for public HTTPS)
//   - webhookURL == "" → long polling (zero-tunnel local / docker compose demo)
func New(token string, users *service.UserService, ag *agent.Agent, log *zap.Logger, webhookPath, webhookURL string) (*Handler, error) {
	h := &Handler{users: users, agent: ag, log: log, path: webhookPath}
	opts := []bot.Option{
		bot.WithDefaultHandler(h.onUpdate),
	}
	b, err := bot.New(token, opts...)
	if err != nil {
		return nil, fmt.Errorf("telegram bot: %w", err)
	}
	h.bot = b
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel

	if webhookURL != "" {
		h.mode = "webhook"
		go b.StartWebhook(ctx)
		return h, nil
	}

	// Polling: drop any leftover webhook so getUpdates works.
	h.mode = "polling"
	_, _ = b.DeleteWebhook(ctx, &bot.DeleteWebhookParams{DropPendingUpdates: false})
	go b.Start(ctx)
	log.Info("telegram delivery: long polling (no TELEGRAM_WEBHOOK_URL)")
	return h, nil
}

func (h *Handler) Mode() string { return h.mode }

func (h *Handler) Close() {
	if h.cancel != nil {
		h.cancel()
	}
}

func (h *Handler) WebhookHandler() http.Handler {
	return h.bot.WebhookHandler()
}

func (h *Handler) SetWebhook(ctx context.Context, url string) error {
	_, err := h.bot.SetWebhook(ctx, &bot.SetWebhookParams{URL: url})
	return err
}

func (h *Handler) onUpdate(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}
	msg := update.Message
	from := msg.From
	text := msg.Text
	chatID := msg.Chat.ID

	display := from.FirstName
	if display == "" {
		display = from.Username
	}
	if display == "" {
		display = fmt.Sprintf("user_%d", from.ID)
	}
	var username *string
	if from.Username != "" {
		u := from.Username
		username = &u
	}

	user, err := h.users.UpsertFromTelegram(ctx, from.ID, display, username, domain.LangRU)
	if err != nil {
		h.log.Error("upsert user", zap.Error(err))
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: "Ошибка регистрации."})
		return
	}

	if text == "/start" {
		reply := fmt.Sprintf("Привет, %s! Я учитываю твои траты. Пиши на естественном языке, например: «кофе 350».\nЯзык: ru/en — напиши «english» или «русский».", user.DisplayName)
		if user.IsFamilyHead {
			reply += "\nТы глава семьи — веб-панель доступна с Basic Auth."
		}
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: reply})
		return
	}

	tc := agent.TrustedContext{
		UserID:   user.ID,
		Language: user.Language,
	}
	res := h.agent.HandleMessage(ctx, chatID, text, tc)
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: res.Reply})
}
