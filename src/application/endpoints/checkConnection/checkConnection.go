package checkconnection

import (
	"time"

	shared "github.com/ChatDetectiveORG/command-handler/src/application/endpoints"
	"github.com/ChatDetectiveORG/command-handler/src/infrastructure/postgresql"
	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	utils "github.com/ChatDetectiveORG/shared/utils"
	tele "gopkg.in/telebot.v4"
)

func NewCheckConnectionEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"check_connection",
		*h.HandlerChain{}.Init(
			30*time.Second,
			h.InitChainHandler(run, h.EndOnError),
		),
		h.Or(
			h.Command([]string{"check_connection"}),
			h.TextCommand(shared.BtnCheckConnection),
		),
	)
	return ep
}

func buildConnectedMessage(chatID int64) *tele.Message {
	return &tele.Message{
		Chat: &tele.Chat{ID: chatID},
		Text: "Бот подключен, все работает как надо!👌\n\n👍Ты можешь получать уведомления, если кто-то удалит или изменит сообщения в личных чатах \n👍Ты можешь скачивать фото, видео, голосовые сообщения и кружочки которые обычно исчезают после одного просмотра\n👍У тебя есть возможность восстановить чат даже после его удаления \n\nВ общем, полный контроль над собеседником!",
		Entities: tele.Entities{
			{Type: tele.EntityBold, Offset: 0, Length: 37},
			{Type: tele.EntityBold, Offset: 37, Length: 2},
			{Type: tele.EntityCustomEmoji, Offset: 37, Length: 2, CustomEmojiID: "5463423955014529788"},
			{Type: tele.EntityBold, Offset: 39, Length: 1},
			{Type: tele.EntityCustomEmoji, Offset: 41, Length: 2, CustomEmojiID: "5465465194056525619"},
			{Type: tele.EntityCustomEmoji, Offset: 132, Length: 2, CustomEmojiID: "5465465194056525619"},
			{Type: tele.EntityCustomEmoji, Offset: 245, Length: 2, CustomEmojiID: "5465465194056525619"},
		},
	}
}

func buildDisconnectedMessage(chatID int64) *tele.Message {
	botMentionOffset := utils.TgLen("Бот отключён!🙈\n\nБольшая часть функций недоступна. Бот будет работать только в тех чатах, где собеседник использует ")
	return &tele.Message{
		Chat: &tele.Chat{ID: chatID},
		Text: "Бот отключён!🙈\n\nБольшая часть функций недоступна. Бот будет работать только в тех чатах, где собеседник использует " + shared.BotUsername,
		Entities: tele.Entities{
			{Type: tele.EntityBold, Offset: 0, Length: 13},
			{Type: tele.EntityBold, Offset: 13, Length: 2},
			{Type: tele.EntityCustomEmoji, Offset: 13, Length: 2, CustomEmojiID: "5463345378587849154"},
			{Type: tele.EntityBold, Offset: 15, Length: 1},
			{Type: tele.EntityMention, Offset: botMentionOffset, Length: utils.TgLen(shared.BotUsername)},
		},
		ReplyMarkup: &tele.ReplyMarkup{
			InlineKeyboard: [][]tele.InlineButton{
				{{Text: "Показать список", Data: shared.UniqueShowContacts}},
			},
		},
	}
}

func run(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	db := postgresql.GetDB()
	chatID := update.Message.Chat.ID

	user, err := shared.GetUserByTgID(db, update.Message.Sender.ID)
	if e.IsNonNil(err) {
		return err
	}

	if user.BusinessConnectionIDHash != "" {
		return hashe.Emit(shared.OutgoingRoutingKey, buildConnectedMessage(chatID))
	}

	if err := hashe.Emit(shared.OutgoingRoutingKey, buildDisconnectedMessage(chatID)); e.IsNonNil(err) {
		return err
	}

	return err
}
