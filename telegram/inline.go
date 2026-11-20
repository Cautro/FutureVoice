package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

func requestActionKeyboard(requestID int64, status RequestStatus) *telego.InlineKeyboardMarkup {
	rows := [][]telego.InlineKeyboardButton{
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Открыть обращение").WithCallbackData(callbackRequestView(requestID)),
		),
	}
	if status == RequestRejected {
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("✍️ Подать апелляцию").WithCallbackData(callbackRequestAppeal(requestID)),
		))
	} else {
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Написать команде").WithCallbackData(callbackRequestReply(requestID)),
		))
	}
	rows = append(rows, tu.InlineKeyboardRow(
		tu.InlineKeyboardButton("Мои обращения").WithCallbackData("requests:mine"),
	))
	return tu.InlineKeyboard(rows...)
}

func notificationKeyboard(requestID int64, status RequestStatus) *telego.InlineKeyboardMarkup {
	return requestActionKeyboard(requestID, status)
}

func requestCardKeyboard(request Request) *telego.InlineKeyboardMarkup {
	rows := [][]telego.InlineKeyboardButton{
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Открыть").WithCallbackData(callbackRequestView(request.ID)),
		),
	}
	if request.Status == RequestRejected {
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("✍️ Подать апелляцию").WithCallbackData(callbackRequestAppeal(request.ID)),
		))
	} else {
		rows = append(rows, tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Написать команде").WithCallbackData(callbackRequestReply(request.ID)),
		))
	}
	return tu.InlineKeyboard(rows...)
}

func callbackRequestView(requestID int64) string  { return fmt.Sprintf("request:view:%d", requestID) }
func callbackRequestReply(requestID int64) string { return fmt.Sprintf("request:reply:%d", requestID) }
func callbackRequestAppeal(requestID int64) string {
	return fmt.Sprintf("request:appeal:%d", requestID)
}

func (a *botApplication) sendMessageWithButtons(ctx *th.Context, chatID int64, text string, keyboard *telego.InlineKeyboardMarkup) error {
	_, err := ctx.Bot().SendMessage(ctx, tu.Message(tu.ID(chatID), text).WithReplyMarkup(keyboard))
	return err
}

func (a *botApplication) sendRequestCards(ctx *th.Context, chatID int64, requests []Request) error {
	if len(requests) == 0 {
		return a.sendMessageWithButtons(ctx, chatID, "У тебя пока нет обращений. Можешь создать первое через меню.", tu.InlineKeyboard(
			tu.InlineKeyboardRow(tu.InlineKeyboardButton("Оставить заявку").WithCallbackData("menu:create")),
		))
	}

	if err := sendMessage(ctx, chatID, "Твои обращения. Нажми «Открыть», чтобы посмотреть подробности и переписку."); err != nil {
		return err
	}
	for _, request := range requests {
		text := fmt.Sprintf(
			"Обращение №%d\n%s\nТема: %s\nСтатус: %s\nСоздано: %s",
			request.ID,
			requestTypeLabel(request.RequestType),
			truncateRunes(request.Theme, 180),
			statusLabel(request.Status),
			request.CreatedAt.Local().Format("02.01.2006 15:04"),
		)
		if err := a.sendMessageWithButtons(ctx, chatID, text, requestCardKeyboard(request)); err != nil {
			return err
		}
	}
	return nil
}

func (a *botApplication) handleCallbackQuery(ctx *th.Context, query telego.CallbackQuery) error {
	if query.Message == nil {
		return ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID).WithText("Это действие уже недоступно."))
	}
	if err := ctx.Bot().AnswerCallbackQuery(ctx, tu.CallbackQuery(query.ID)); err != nil {
		return err
	}

	chatID := query.Message.GetChat().ID
	telegramID := query.From.ID
	parts := strings.Split(query.Data, ":")
	if len(parts) == 2 && parts[0] == "requests" && parts[1] == "mine" {
		a.conversations.Clear(telegramID)
		requests, err := a.backend.ListRequests(ctx, telegramID, false, 100)
		if err != nil {
			return sendMessage(ctx, chatID, backendErrorMessage(err))
		}
		return a.sendRequestCards(ctx, chatID, requests)
	}
	if len(parts) == 2 && parts[0] == "reply" && parts[1] == "cancel" {
		a.conversations.Clear(telegramID)
		return sendMessage(ctx, chatID, "Ответ отменён. Никакое сообщение не отправлено.")
	}
	if len(parts) == 2 && parts[0] == "menu" && parts[1] == "create" {
		a.conversations.Set(telegramID, requestDraft{Stage: stageChooseType})
		return a.sendTypeMenu(ctx, chatID, "Выбери тип обращения.")
	}
	if len(parts) != 3 || parts[0] != "request" {
		return sendMessage(ctx, chatID, "Кнопка устарела. Открой «Мои заявки» и выбери обращение заново.")
	}
	requestID, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || requestID <= 0 {
		return sendMessage(ctx, chatID, "Не удалось определить обращение.")
	}

	selected, err := a.backend.GetRequest(ctx, telegramID, requestID)
	if err != nil {
		return sendMessage(ctx, chatID, backendErrorMessage(err))
	}

	switch parts[1] {
	case "view":
		messages, err := a.backend.ListRequestMessages(ctx, telegramID, requestID)
		if err != nil {
			return sendMessage(ctx, chatID, backendErrorMessage(err))
		}
		text := formatRequestDetails(*selected, messages)
		return a.sendMessageWithButtons(ctx, chatID, text, requestActionKeyboard(requestID, selected.Status))
	case "reply", "appeal":
		appeal := parts[1] == "appeal"
		if appeal && selected.Status != RequestRejected {
			return sendMessage(ctx, chatID, "Апелляцию можно подать только по отклонённому обращению.")
		}
		if appeal {
			a.conversations.Set(telegramID, requestDraft{Stage: stageReplyMessage, RequestID: requestID, Status: selected.Status, IsAppeal: true})
			return a.sendMessageWithButtons(ctx, chatID, fmt.Sprintf("Напиши апелляцию по обращению №%d. Объясни, почему решение стоит пересмотреть. Текст увидит команда модерации.", requestID), replyCancelKeyboard())
		}
		a.conversations.Set(telegramID, requestDraft{Stage: stageReplyMessage, RequestID: requestID, Status: selected.Status})
		return a.sendMessageWithButtons(ctx, chatID, fmt.Sprintf("Напиши сообщение по обращению №%d. Оно попадёт в общую переписку с командой.", requestID), replyCancelKeyboard())
	default:
		return sendMessage(ctx, chatID, "Неизвестное действие. Открой обращение заново.")
	}
}

func replyCancelKeyboard() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(tu.InlineKeyboardRow(
		tu.InlineKeyboardButton("Отмена").WithCallbackData("reply:cancel"),
	))
}

func formatRequestDetails(request Request, messages []RequestMessage) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "Обращение №%d · %s\n", request.ID, statusLabel(request.Status))
	fmt.Fprintf(&builder, "Тема: %s\nТип: %s\nСоздано: %s\n\n", request.Theme, requestTypeLabel(request.RequestType), request.CreatedAt.Local().Format("02.01.2006 15:04"))
	if request.Description != "" {
		builder.WriteString("Описание:\n")
		builder.WriteString(truncateRunes(request.Description, 900))
		builder.WriteString("\n\n")
	}
	if len(messages) == 0 {
		builder.WriteString("Пока сообщений нет.")
		return builder.String()
	}
	builder.WriteString(formatMessages(request.ID, messages))
	return truncateRunes(builder.String(), 3600)
}
