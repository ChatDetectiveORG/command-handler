package businessconnection

import (
	"github.com/ChatDetectiveORG/shared/telegram"
	tele "gopkg.in/telebot.v4"
)

func buildConnectedMessage(chatID int64) *tele.Message {
	messageBuilder := telegram.MessageBuilder{Mdv2Enabled: true}

	messageBuilder.WriteString(
		"Бот подключен, все работает как надо!", telegram.TextFormat{Type: telegram.TextFormatTypeBold},
	).WriteString(
		"👌",  telegram.TextFormat{Type: telegram.TextFormatTypeLink}.WithCustomEmojiID("5463423955014529788"),
	).WriteString(
		"\n\nТеперь:\n",
	).WriteString(
		"👍", telegram.TextFormat{Type: telegram.TextFormatTypeLink}.WithCustomEmojiID("5465465194056525619"),
	).WriteString(
		"Ты будешь получать уведомления, если кто-то удалит или изменит сообщения в личных чатах \n",
	).WriteString(
		"👍", telegram.TextFormat{Type: telegram.TextFormatTypeLink}.WithCustomEmojiID("5465465194056525619"),
	).WriteString(
		"Ты сможешь скачивать фото, видео, голосовые сообщения и кружочки которые обычно исчезают после одного просмотра\n",
	).WriteString(
		"👍", telegram.TextFormat{Type: telegram.TextFormatTypeLink}.WithCustomEmojiID("5465465194056525619"),
	).WriteString(
		"У тебя будет возможность восстановить чат даже после его удаления \n\nВ общем, полный контроль над собеседником!",
	)

	return messageBuilder.Build(chatID)
}
