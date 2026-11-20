package main

import (
	"strings"
	"testing"
)

func TestFormatMessagesUsesInlineReplyHint(t *testing.T) {
	messages := []RequestMessage{{
		ID: 1, RequestID: 42, AuthorName: "Ученик", AuthorRole: "student",
		Body: "Подскажите, есть новости?",
	}}
	formatted := formatMessages(42, messages)
	if !strings.Contains(formatted, "Переписка по обращению №42") || !strings.Contains(formatted, "Подскажите, есть новости?") {
		t.Fatalf("conversation output is incomplete: %s", formatted)
	}
	if strings.Contains(formatted, "/reply") || strings.Contains(formatted, "/messages") {
		t.Fatalf("conversation output exposes slash commands: %s", formatted)
	}
}

func TestFormatMessagesStaysWithinTelegramLimit(t *testing.T) {
	messages := make([]RequestMessage, 10)
	for i := range messages {
		messages[i] = RequestMessage{
			ID: int64(i + 1), RequestID: 42, AuthorName: "Ученик", AuthorRole: "student",
			Body: strings.Repeat("длинное сообщение ", 300),
		}
	}
	formatted := formatMessages(42, messages)
	if got := len([]rune(formatted)); got > 4096 {
		t.Fatalf("formatMessages() produced %d runes, Telegram limit is 4096", got)
	}
	if !strings.Contains(formatted, "История сокращена") && !strings.Contains(formatted, "Сообщение сокращено") {
		t.Fatalf("long conversation was not marked as shortened: %s", formatted)
	}
}
