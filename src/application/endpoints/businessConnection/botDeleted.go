package businessconnection

import (
	constants "github.com/ChatDetectiveORG/shared/constants"
	. "github.com/ChatDetectiveORG/shared/messageBuilder"
	tele "gopkg.in/telebot.v4"
)

func buildDisconnectedMessage(chatID int64) *tele.Message {
	messageBuilder := MessageBuilder{Mdv2Enabled: true}
	messageBuilder.Write(
		B(T("Бот отключён!", Args{NoNewline: true})), E("5463345378587849154"),
		T("Теперь большая часть функций недоступна."),
		T(""),
		I(T("Для того, чтобы снова получать оригиналы удалённых и изменённых сообщений, а также иметь возможность восстановить удалённые чаты, подключи "+constants.BotUsername)),
	)

	return messageBuilder.Build(chatID)
}
