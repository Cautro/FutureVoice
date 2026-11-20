package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"
	"unicode/utf8"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

const (
	menuCreateRequest = "Оставить заявку"
	menuMyRequests    = "Мои заявки"
	menuCancel        = "Отмена"

	requestThemeMinLength       = 3
	requestThemeMaxLength       = 120
	requestDescriptionMinLength = 10
	requestDescriptionMaxLength = 5000
)

type botApplication struct {
	backend       *BackendClient
	conversations *conversationStore
	mainMenu      *telego.ReplyKeyboardMarkup
	adminMenu     *telego.ReplyKeyboardMarkup
	typeMenu      *telego.ReplyKeyboardMarkup
}

func newBotApplication(backend *BackendClient, conversations *conversationStore) *botApplication {
	return &botApplication{
		backend:       backend,
		conversations: conversations,
		mainMenu: tu.Keyboard(
			tu.KeyboardRow(tu.KeyboardButton(menuCreateRequest)),
			tu.KeyboardRow(tu.KeyboardButton(menuMyRequests)),
		).WithResizeKeyboard(),
		adminMenu: tu.Keyboard(
			tu.KeyboardRow(tu.KeyboardButton(menuCreateRequest)),
			tu.KeyboardRow(tu.KeyboardButton(menuMyRequests)),
		).WithResizeKeyboard(),
		typeMenu: tu.Keyboard(
			tu.KeyboardRow(tu.KeyboardButton("Жалоба"), tu.KeyboardButton("Предложение")),
			tu.KeyboardRow(tu.KeyboardButton("Предложение по 3D-печати")),
			tu.KeyboardRow(tu.KeyboardButton(menuCancel)),
		).WithResizeKeyboard(),
	}
}

func (a *botApplication) registerHandlers(handlers *th.BotHandler) {
	handlers.Use(
		th.PanicRecoveryHandler(func(recovered any) error {
			slog.Error(
				"panic while processing Telegram update",
				"panic", recovered,
				"stack", string(debug.Stack()),
			)
			return fmt.Errorf("Telegram update handler panic: %v", recovered)
		}),
		a.serializeUserUpdates,
	)

	handlers.HandleMessage(a.handleStart, th.CommandEqual("start"))
	handlers.HandleMessage(a.handleMenu, th.CommandEqual("menu"))
	handlers.HandleMessage(a.handleCancel, th.CommandEqual("cancel"))
	handlers.HandleMessage(a.handleHelp, th.CommandEqual("help"))
	handlers.HandleCallbackQuery(a.handleCallbackQuery, th.AnyCallbackQueryWithMessage())
	handlers.HandleMessage(a.handleUnknownCommand, th.AnyCommand())
	handlers.HandleMessage(a.handleTextMessage, th.AnyMessage())
}

func (a *botApplication) serializeUserUpdates(ctx *th.Context, update telego.Update) error {
	var telegramID int64
	if update.Message != nil && update.Message.From != nil {
		telegramID = update.Message.From.ID
	} else if update.CallbackQuery != nil {
		telegramID = update.CallbackQuery.From.ID
	}
	if telegramID <= 0 {
		return ctx.Next(update)
	}

	unlock := a.conversations.LockUser(telegramID)
	defer unlock()
	return ctx.Next(update)
}

func (a *botApplication) handleStart(ctx *th.Context, message telego.Message) error {
	if message.From == nil {
		return sendMessage(ctx, message.Chat.ID, "Не удалось определить отправителя.")
	}

	registered, err := a.registerTelegramUser(ctx, message.From)
	if err != nil {
		slog.Error("register Telegram user", "telegram_id", message.From.ID, "error", err)
		return sendMessage(ctx, message.Chat.ID, backendErrorMessage(err))
	}
	if !registered.IsActive {
		return sendMessage(ctx, message.Chat.ID, "Доступ к боту для этого аккаунта отключён. Обратись к администратору.")
	}

	welcome := "Привет, " + safeName(message.From.FirstName) + "!\n" +
		"Здесь можно отправлять предложения и жалобы школьному самоуправлению.\n\n" +
		"Статусы, ответы и апелляции будут приходить сюда. Открывай заявки кнопками под сообщениями."
	if registered.Role == "admin" || registered.Role == "moderator" {
		welcome += "\n\nДля работы с обращениями используй панель FutureVoice."
		return a.sendAdminMenu(ctx, message.Chat.ID, welcome)
	}
	return a.sendMenu(ctx, message.Chat.ID, welcome)
}

func (a *botApplication) handleMenu(ctx *th.Context, message telego.Message) error {
	if message.From == nil {
		return sendMessage(ctx, message.Chat.ID, "Не удалось определить отправителя.")
	}
	return a.sendMenuForUser(ctx, message.Chat.ID, message.From, "Выбери действие в меню.")
}

func (a *botApplication) handleCancel(ctx *th.Context, message telego.Message) error {
	if message.From == nil {
		return sendMessage(ctx, message.Chat.ID, "Не удалось определить отправителя.")
	}
	a.conversations.Clear(message.From.ID)
	return a.sendMenuForUser(ctx, message.Chat.ID, message.From, "Текущее действие отменено.")
}

func (a *botApplication) handleHelp(ctx *th.Context, message telego.Message) error {
	text := "Доступные действия:\n" +
		menuCreateRequest + " — создать обращение\n" +
		menuMyRequests + " — посмотреть свои обращения\n" +
		"Чтобы написать команде или подать апелляцию, открой обращение кнопкой под списком или уведомлением.\n" +
		"/cancel — отменить заполнение"

	if message.From != nil {
		registered, err := a.registerTelegramUser(ctx, message.From)
		if err != nil {
			return sendMessage(ctx, message.Chat.ID, backendErrorMessage(err))
		}
		if registered.Role == "admin" || registered.Role == "moderator" {
			text += "\n\nДля изменения статусов и ответов пользователям открой веб-панель FutureVoice."
		}
	}
	return sendMessage(ctx, message.Chat.ID, text)
}

func (a *botApplication) handleUnknownCommand(ctx *th.Context, message telego.Message) error {
	return sendMessage(ctx, message.Chat.ID, "Неизвестная команда. Используй кнопки меню или /help.")
}

func (a *botApplication) handleTextMessage(ctx *th.Context, message telego.Message) error {
	if message.From == nil || message.Text == "" {
		return nil
	}

	telegramID := message.From.ID
	text := strings.TrimSpace(message.Text)

	switch text {
	case menuCreateRequest:
		a.conversations.Set(telegramID, requestDraft{Stage: stageChooseType})
		return a.sendTypeMenu(ctx, message.Chat.ID, "Выбери тип обращения.")
	case menuMyRequests:
		if draft, exists := a.conversations.Get(telegramID); exists && draft.Stage == stageReplyMessage {
			a.conversations.Clear(telegramID)
		}
		requests, err := a.backend.ListRequests(ctx, telegramID, false, 20)
		if err != nil {
			return sendMessage(ctx, message.Chat.ID, backendErrorMessage(err))
		}
		return a.sendRequestCards(ctx, message.Chat.ID, requests)
	case menuCancel:
		a.conversations.Clear(telegramID)
		return a.sendMenuForUser(ctx, message.Chat.ID, message.From, "Текущее действие отменено.")
	}

	draft, ok := a.conversations.Get(telegramID)
	if !ok {
		return sendMessage(ctx, message.Chat.ID, "Не понял действие. Используй кнопки меню или /help.")
	}

	switch draft.Stage {
	case stageChooseType:
		requestType, valid := parseRequestType(text)
		if !valid {
			return a.sendTypeMenu(ctx, message.Chat.ID, "Выбери один из типов кнопками ниже или нажми «Отмена».")
		}
		draft.RequestType = requestType
		draft.Stage = stageEnterTheme
		a.conversations.Set(telegramID, draft)
		return sendMessage(ctx, message.Chat.ID, "Напиши краткую тему обращения (от 3 до 120 символов).")

	case stageEnterTheme:
		if length := utf8.RuneCountInString(text); length < requestThemeMinLength || length > requestThemeMaxLength {
			return sendMessage(ctx, message.Chat.ID, "Тема должна содержать от 3 до 120 символов. Попробуй ещё раз.")
		}
		draft.Theme = text
		draft.Stage = stageEnterDescription
		a.conversations.Set(telegramID, draft)
		return sendMessage(ctx, message.Chat.ID, "Теперь подробно опиши обращение (от 10 до 5000 символов).")

	case stageEnterDescription:
		if length := utf8.RuneCountInString(text); length < requestDescriptionMinLength || length > requestDescriptionMaxLength {
			return sendMessage(ctx, message.Chat.ID, "Описание должно содержать от 10 до 5000 символов. Попробуй ещё раз.")
		}
		return a.submitRequest(ctx, message)

	case stageSubmitting:
		return sendMessage(ctx, message.Chat.ID, "Заявка уже отправляется. Подожди ответ сервера.")

	case stageReplyMessage:
		maxReplyLength := 4000
		if draft.IsAppeal {
			maxReplyLength = 3900 // Keep the appeal prefix below the backend's 4000-rune limit.
		}
		if length := utf8.RuneCountInString(text); length < 1 || length > maxReplyLength {
			return sendMessage(ctx, message.Chat.ID, fmt.Sprintf("Сообщение должно содержать от 1 до %d символов.", maxReplyLength))
		}
		body := text
		if draft.IsAppeal {
			body = "Апелляция по отклонённой заявке:\n\n" + text
		}
		if err := a.backend.AddRequestMessage(ctx, telegramID, draft.RequestID, body); err != nil {
			return sendMessage(ctx, message.Chat.ID, backendErrorMessage(err))
		}
		a.conversations.Clear(telegramID)
		return a.sendMessageWithButtons(ctx, message.Chat.ID, fmt.Sprintf("Сообщение по обращению №%d отправлено команде. Ответ придёт сюда же.", draft.RequestID), requestActionKeyboard(draft.RequestID, draft.Status))

	default:
		a.conversations.Clear(telegramID)
		return a.sendMenuForUser(ctx, message.Chat.ID, message.From, "Состояние формы сброшено. Начни заново через меню.")
	}
}

func (a *botApplication) submitRequest(ctx *th.Context, message telego.Message) error {
	telegramID := message.From.ID
	inFlight, started := a.conversations.BeginSubmission(telegramID, message.MessageID)
	if !started {
		return sendMessage(ctx, message.Chat.ID, "Заявка уже отправляется или форма устарела. Используй /cancel и начни заново.")
	}

	created, err := a.backend.CreateRequest(ctx, CreateRequestInput{
		TelegramID:     telegramID,
		IdempotencyKey: inFlight.IdempotencyKey,
		RequestType:    inFlight.RequestType,
		Theme:          inFlight.Theme,
		Description:    strings.TrimSpace(message.Text),
	})
	if err != nil {
		a.conversations.RetrySubmission(telegramID, inFlight)
		slog.Error("create request", "telegram_id", telegramID, "error", err)
		return sendMessage(ctx, message.Chat.ID, backendErrorMessage(err)+"\nПовтори отправку того же описания или используй /cancel.")
	}

	confirmation := fmt.Sprintf(
		"Заявка №%d успешно создана. Её текущий статус: %s.\nКогда появятся новости, мы сообщим сюда.",
		created.ID,
		statusLabel(created.Status),
	)
	if err := a.sendMessageWithButtons(ctx, message.Chat.ID, confirmation, requestActionKeyboard(created.ID, created.Status)); err != nil {
		// Keep the same idempotency key so the user can safely retry if Telegram
		// accepted the request but the confirmation message could not be delivered.
		a.conversations.RetrySubmission(telegramID, inFlight)
		return err
	}
	a.conversations.Clear(telegramID)
	return nil
}

func (a *botApplication) registerTelegramUser(ctx context.Context, from *telego.User) (*TelegramUserResponse, error) {
	if from == nil {
		return nil, errors.New("Telegram update has no sender")
	}
	return a.backend.RegisterTelegramUser(ctx, TelegramUserRequest{
		TelegramID: from.ID,
		Username:   from.Username,
		FirstName:  from.FirstName,
		LastName:   from.LastName,
	})
}

func (a *botApplication) sendMenuForUser(ctx *th.Context, chatID int64, from *telego.User, text string) error {
	registered, err := a.registerTelegramUser(ctx, from)
	if err != nil {
		return sendMessage(ctx, chatID, backendErrorMessage(err))
	}
	if !registered.IsActive {
		return sendMessage(ctx, chatID, "Доступ к боту для этого аккаунта отключён.")
	}
	if registered.Role == "admin" || registered.Role == "moderator" {
		return a.sendAdminMenu(ctx, chatID, text)
	}
	return a.sendMenu(ctx, chatID, text)
}

func (a *botApplication) sendMenu(ctx *th.Context, chatID int64, text string) error {
	_, err := ctx.Bot().SendMessage(ctx, tu.Message(tu.ID(chatID), text).WithReplyMarkup(a.mainMenu))
	return err
}

func (a *botApplication) sendAdminMenu(ctx *th.Context, chatID int64, text string) error {
	_, err := ctx.Bot().SendMessage(ctx, tu.Message(tu.ID(chatID), text).WithReplyMarkup(a.adminMenu))
	return err
}

func (a *botApplication) sendTypeMenu(ctx *th.Context, chatID int64, text string) error {
	_, err := ctx.Bot().SendMessage(ctx, tu.Message(tu.ID(chatID), text).WithReplyMarkup(a.typeMenu))
	return err
}

func sendMessage(ctx *th.Context, chatID int64, text string) error {
	_, err := ctx.Bot().SendMessage(ctx, tu.Message(tu.ID(chatID), text))
	return err
}
