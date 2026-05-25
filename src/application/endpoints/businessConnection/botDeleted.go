package businessconnection

import (
	shared "github.com/ChatDetectiveORG/command-handler/src/application/endpoints"
	"github.com/ChatDetectiveORG/shared/telegram"
	tele "gopkg.in/telebot.v4"
)

func buildDisconnectedMessage(chatID int64) *tele.Message {
	messageBuilder := telegram.MessageBuilder{Mdv2Enabled: true}
	messageBuilder.WriteString(
		"Бот отключён!", telegram.TextFormat{Type: telegram.TextFormatTypeBold},
	).WriteString(
		"🙈", telegram.TextFormat{Type: telegram.TextFormatTypeLink}.WithCustomEmojiID("5463345378587849154"),
	).WriteString(
		"Теперь большая часть функций недоступна.\n\n",
	).WriteString(
		"Для того, чтобы снова получать оригиналы удалённых и изменённых сообщений, а также иметь возможность восстановить удалённые чаты, подключи ", telegram.TextFormat{Type: telegram.TextFormatTypeItalic},
	).WriteString(shared.BotUsername)

	return messageBuilder.Build(chatID)
}
