package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// runNotificationWorker delivers status changes and staff replies queued by the backend.
// The backend keeps failed messages and retries them with increasing delays.
func (a *botApplication) runNotificationWorker(ctx context.Context, bot *telego.Bot) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		a.deliverNotifications(ctx, bot)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (a *botApplication) deliverNotifications(ctx context.Context, bot *telego.Bot) {
	items, err := a.backend.ListTelegramNotifications(ctx, 20)
	if err != nil {
		if ctx.Err() == nil {
			slog.Warn("load pending Telegram notifications", "error", err)
		}
		return
	}

	for _, item := range items {
		if ctx.Err() != nil {
			return
		}
		message := tu.Message(tu.ID(item.TelegramID), item.Body).WithReplyMarkup(notificationKeyboard(item.RequestID, item.Status))
		_, sendErr := bot.SendMessage(ctx, message)
		if sendErr != nil {
			slog.Warn("deliver Telegram notification", "notification_id", item.ID, "request_id", item.RequestID, "error", sendErr)
			if err := a.backend.FailTelegramNotification(ctx, item.ID, sendErr); err != nil && ctx.Err() == nil {
				slog.Error("reschedule failed Telegram notification", "notification_id", item.ID, "error", err)
			}
			continue
		}
		if err := a.backend.MarkTelegramNotificationDelivered(ctx, item.ID); err != nil && ctx.Err() == nil {
			slog.Error("acknowledge delivered Telegram notification", "notification_id", item.ID, "error", err)
		}
	}
}
