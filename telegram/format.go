package main

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

func safeName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "ученик"
	}
	return name
}

func parseRequestType(value string) (RequestType, bool) {
	switch value {
	case "Жалоба":
		return ComplaintType, true
	case "Предложение":
		return OfferType, true
	case "Предложение по 3D-печати":
		return Print3DOfferType, true
	default:
		return "", false
	}
}

func idempotencyKey(telegramID int64, messageID int) string {
	return fmt.Sprintf("tg:%d:%d", telegramID, messageID)
}

func formatMessages(requestID int64, messages []RequestMessage) string {
	if len(messages) == 0 {
		return fmt.Sprintf("В переписке по обращению №%d пока нет сообщений. Нажми кнопку ниже, чтобы продолжить общение с командой.", requestID)
	}

	const maxTelegramRunes = 3500
	const maxMessageRunes = 1200

	var builder strings.Builder
	fmt.Fprintf(&builder, "Переписка по обращению №%d:\n\n", requestID)
	for _, message := range messages {
		author := message.AuthorName
		if message.AuthorRole == "admin" || message.AuthorRole == "moderator" {
			author = "Команда FutureVoice"
		}
		body := truncateRunes(message.Body, maxMessageRunes)
		entry := fmt.Sprintf("%s · %s:\n%s\n\n", safeName(author), message.CreatedAt.Local().Format("02.01 15:04"), body)
		if utf8.RuneCountInString(builder.String())+utf8.RuneCountInString(entry) > maxTelegramRunes-100 {
			builder.WriteString("История сокращена. Используй кнопку под сообщением, чтобы продолжить общение.")
			break
		}
		builder.WriteString(entry)
		if body != message.Body {
			builder.WriteString("(Сообщение сокращено.)\n\n")
		}
	}
	return builder.String()
}

func truncateRunes(value string, max int) string {
	if max < 0 {
		return ""
	}
	if utf8.RuneCountInString(value) <= max {
		return value
	}
	runes := []rune(value)
	return string(runes[:max]) + "…"
}

func requestTypeLabel(requestType RequestType) string {
	switch requestType {
	case ComplaintType:
		return "жалоба"
	case OfferType:
		return "предложение"
	case Print3DOfferType:
		return "3D-печать"
	default:
		return string(requestType)
	}
}

func statusLabel(status RequestStatus) string {
	switch status {
	case RequestOpen:
		return "открыта"
	case RequestInProgress:
		return "в работе"
	case RequestWaitingForUser:
		return "ожидает твоего ответа"
	case RequestClosed:
		return "завершена"
	case RequestRejected:
		return "отклонена"
	default:
		return string(status)
	}
}

func backendErrorMessage(err error) string {
	var backendErr *BackendHTTPError
	if errors.As(err, &backendErr) {
		switch backendErr.StatusCode {
		case 400:
			return "Проверь введённые данные."
		case 401:
			return "Не удалось авторизовать запрос к серверу. Обратись к администратору бота."
		case 403:
			return "У тебя нет прав для выполнения этого действия."
		case 404:
			return "Заявка или пользователь не найдены."
		case 409:
			return "Заявку нельзя изменить в её текущем состоянии."
		case 503:
			return "Сервис временно недоступен. Попробуй позже."
		}
	}
	return "Не удалось связаться с сервером. Попробуй позже."
}
