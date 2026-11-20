package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"future-voice/internal/config"

	"github.com/joho/godotenv"
	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
)

func main() {
	if err := godotenv.Load(); err != nil {
		slog.Info("No .env file found; reading environment variables")
	}

	cfg, err := config.LoadTelegram()
	if err != nil {
		slog.Error("load Telegram config", "error", err)
		os.Exit(1)
	}

	bot, err := telego.NewBot(cfg.BotToken)
	if err != nil {
		slog.Error("create Telegram bot", "error", err)
		os.Exit(1)
	}

	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	updates, err := bot.UpdatesViaLongPolling(rootCtx, nil)
	if err != nil {
		slog.Error("start Telegram long polling", "error", err)
		os.Exit(1)
	}

	telegramHandlers, err := th.NewBotHandler(bot, updates)
	if err != nil {
		slog.Error("create Telegram handlers", "error", err)
		os.Exit(1)
	}
	defer func() { _ = telegramHandlers.Stop() }()

	conversations := newConversationStore()
	go conversations.runCleanup(rootCtx, 5*time.Minute)

	backend := NewBackendClient(cfg.BackendURL, cfg.BotAPISecret)
	app := newBotApplication(backend, conversations)
	app.registerHandlers(telegramHandlers)
	go app.runNotificationWorker(rootCtx, bot)

	slog.Info("Telegram bot started")
	if err := telegramHandlers.Start(); err != nil && rootCtx.Err() == nil {
		slog.Error("Telegram handlers stopped unexpectedly", "error", err)
	}
}
