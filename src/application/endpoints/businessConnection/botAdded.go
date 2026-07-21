package businessconnection

import (
	. "github.com/ChatDetectiveORG/shared/messageBuilder"
	tele "gopkg.in/telebot.v4"
)


func buildConnectedMessage(chatID int64) *tele.Message {
	messageBuilder := MessageBuilder{Mdv2Enabled: true}

	messageBuilder.Write(
		B(T("Бот подключен, все работает как надо!", Args{NoNewline: true})), E("5463423955014529788", "👌"),
		T("\n"),
		T("nТеперь:"),
		E("5465465194056525619"), T("Ты будешь получать уведомления, если кто-то удалит или изменит сообщения в личных чатах"),
		E("5465465194056525619"), T("Ты сможешь скачивать фото, видео, голосовые сообщения и кружочки которые обычно исчезают после одного просмотра"),
		E("5465465194056525619"), T("У тебя будет возможность восстановить чат даже после его удаления"),
		T(""),
		T("В общем, полный контроль над собеседником!"),
	)

	return messageBuilder.Build(chatID)
}
