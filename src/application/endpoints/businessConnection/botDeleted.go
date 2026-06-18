package businessconnection

import (
	constants "github.com/ChatDetectiveORG/shared/constants"
	"github.com/ChatDetectiveORG/shared/telegram"
	tele "gopkg.in/telebot.v4"
)

func buildDisconnectedMessage(chatID int64) *tele.Message {
	messageBuilder := telegram.MessageBuilder{Mdv2Enabled: true}
	messageBuilder.WriteString(
		"Бот отключён!", telegram.TextFormat{Type: telegram.Bold},
	).WriteString(
		"🙈", telegram.TextFormat{Type: telegram.Link}.WithCustomEmojiID("5463345378587849154"),
	).WriteString(
		"\nТеперь большая часть функций недоступна.\n\n",
	).WriteString(
		"Для того, чтобы снова получать оригиналы удалённых и изменённых сообщений, а также иметь возможность восстановить удалённые чаты, подключи ", telegram.TextFormat{Type: telegram.Italic},
	).WriteString(constants.BotUsername)

	return messageBuilder.Build(chatID)
}
