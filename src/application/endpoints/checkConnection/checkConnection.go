package checkconnection

import (
	"time"

	"github.com/ChatDetectiveORG/command-handler/src/infrastructure/postgresql"
	helpers "github.com/ChatDetectiveORG/shared/commandHandlerUtils"
	constants "github.com/ChatDetectiveORG/shared/constants"
	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	telegram "github.com/ChatDetectiveORG/shared/messageBuilder"
	tele "gopkg.in/telebot.v4"
)

// Send user info about bot status
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
			h.TextCommand(constants.BtnCheckConnection),
		),
	)
	return ep
}

// Build message about bot connected
//
// Takes: chat ID
//
// Returns: message
func buildConnectedMessage(chatID int64) *tele.Message {
	messageBuilder := telegram.MessageBuilder{Mdv2Enabled: true}

	messageBuilder.WriteString(
		"Бот подключен, все работает как надо!", telegram.TextFormat{Type: telegram.FormatBold},
	).WriteString(
		"👌", telegram.TextFormat{Type: telegram.FormatLink}.WithCustomEmojiID("5463423955014529788"),
	).WriteString(
		"\n\nТеперь:\n",
	).WriteString(
		"👍", telegram.TextFormat{Type: telegram.FormatLink}.WithCustomEmojiID("5465465194056525619"),
	).WriteString(
		"Ты будешь получать уведомления, если кто-то удалит или изменит сообщения в личных чатах \n",
	).WriteString(
		"👍", telegram.TextFormat{Type: telegram.FormatLink}.WithCustomEmojiID("5465465194056525619"),
	).WriteString(
		"Ты сможешь скачивать фото, видео, голосовые сообщения и кружочки которые обычно исчезают после одного просмотра\n",
	).WriteString(
		"👍", telegram.TextFormat{Type: telegram.FormatLink}.WithCustomEmojiID("5465465194056525619"),
	).WriteString(
		"У тебя будет возможность восстановить чат даже после его удаления \n\nВ общем, полный контроль над собеседником!",
	)

	return messageBuilder.Build(chatID)
}

// Build message about bot disconnected
//
// Takes: chat ID
//
// Returns: message
func buildDisconnectedMessage(chatID int64) *tele.Message {
	messageBuilder := telegram.MessageBuilder{Mdv2Enabled: true}

	messageBuilder.WriteString(
		"Бот отключён!", telegram.TextFormat{Type: telegram.FormatBold},
	).WriteString(
		"🙈", telegram.TextFormat{Type: telegram.FormatLink}.WithCustomEmojiID("5463345378587849154"),
	).WriteString(
		"\n\nБольшая часть функций недоступна. Бот будет работать только в тех чатах, где собеседник использует ",
	).WriteString(constants.BotUsername)

	return messageBuilder.Build(chatID)
}

func run(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	db := postgresql.GetDB()
	chatID := update.Message.Chat.ID

	user, err := helpers.GetUserByTgID(db, update.Message.Sender.ID)
	if e.IsNonNil(err) {
		return err
	}

	if user.BusinessConnectionIDHash != "" {
		return hashe.WithParseMode(true).Emit(constants.OutgoingRoutingKey, buildConnectedMessage(chatID))
	}

	if err := hashe.WithParseMode(true).Emit(constants.OutgoingRoutingKey, buildDisconnectedMessage(chatID)); e.IsNonNil(err) {
		return err
	}

	return err
}
