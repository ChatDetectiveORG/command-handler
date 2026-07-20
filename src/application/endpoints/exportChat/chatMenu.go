package exportchat

import (
	"strconv"
	"time"

	"github.com/ChatDetectiveORG/command-handler/src/infrastructure/postgresql"
	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	telegram "github.com/ChatDetectiveORG/shared/messageBuilder"
	models "github.com/ChatDetectiveORG/shared/postgresModels"
	"github.com/ChatDetectiveORG/shared/utils"

	tele "gopkg.in/telebot.v4"

	helpers "github.com/ChatDetectiveORG/shared/commandHandlerUtils"
	constants "github.com/ChatDetectiveORG/shared/constants"
)

func NewViewChatEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"view_chat",
		*h.HandlerChain{}.Init(
			30*time.Second,
			h.InitChainHandler(runViewChat, h.EndOnError),
		),
		h.CallbackStartsWith(constants.UniqueGoToChat),
	)
	return ep
}

// buildMessage builds the message for the chat preview.
func buildMessage(messageID int, chatID int64, sender *models.Telegramuser, interlocutor *models.Telegramuser, code string) (*tele.Message, *e.ErrorInfo) {
	db := postgresql.GetDB()

	tgID, err := interlocutor.GetTgId()
	if e.IsNonNil(err) {
		return nil, err.PushStack()
	}

	fullName, err := interlocutor.GetFullName()
	if e.IsNonNil(err) {
		return nil, err.PushStack()
	}

	count, err := chatMessageCount(db, sender, interlocutor)
	if e.IsNonNil(err) {
		return nil, err.PushStack()
	}

	messageBuilder := telegram.MessageBuilder{Mdv2Enabled: true}

	messageBuilder.WriteString(
		"Чат с ", telegram.TextFormat{Type: telegram.FormatBold},
	).WriteString(
		fullName, telegram.TextFormat{Type: telegram.FormatLink}.WithUserMention(tgID), telegram.TextFormat{Type: telegram.FormatBold},
	).WriteString("\n")

	if count > 0 {
		messageBuilder.WriteString(
			"Возможно восстановить "+strconv.Itoa(count)+" сообщений", telegram.TextFormat{Type: telegram.FormatItalic},
		)
	} else {
		messageBuilder.WriteString(
			"К сожалению, чат не может быть восстановлен.", telegram.TextFormat{Type: telegram.FormatItalic},
		).WriteString(
			"Почему?", telegram.TextFormat{Type: telegram.FormatLink, URL: "https://t.me/chatdetective_support/1210"},
		)
	}

	messageBuilder.AddButton(tele.InlineButton{Text: "Назад", Data: constants.UniqueChatSelectPage})
	messageBuilder.NextRow()
	if count > 0 {
		messageBuilder.AddButton(tele.InlineButton{Text: "Восстановить", Data: utils.DumpCallbackData(constants.UniqueRestoreChat, map[string]any{constants.CallbackFieldCode: code})})
	}

	return messageBuilder.WithMessageID(messageID).Build(chatID), e.Nil()
}

func runViewChat(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	if update.Callback == nil {
		return e.NewError("missing callback", "view_chat requires callback").WithSeverity(e.Notice)
	}

	db := postgresql.GetDB()
	sender, err := helpers.GetUserByTgID(db, update.Callback.Sender.ID)
	if e.IsNonNil(err) {
		return err.PushStack()
	}

	parsedData := utils.ParseCallbackData(update.Callback.Data)
	code := parsedData[constants.CallbackFieldCode]
	if code == "" {
		return e.NewError("missing interlocutor code", "callback data has no chat code").WithSeverity(e.Notice)
	}

	interlocutor := &models.Telegramuser{}
	if eRaw := db.Model(interlocutor).Where("referral_code = ?", code).Select(); e.IsNonNil(eRaw) {
		return e.FromError(eRaw, "failed to load interlocutor by referral code").WithSeverity(e.Notice)
	}

	err = checkCallbackPermission(sender, interlocutor, db)
	if e.IsNonNil(err) {
		err = hashe.EmitCallback(constants.OutgoingRoutingKey, update.Callback, helpers.AnswerCallbackBanner("У вас нет доступа к этой странице", update.Callback))

		return err
	}

	msg, err := buildMessage(update.Callback.Message.ID, update.Callback.Message.Chat.ID, sender, interlocutor, code)
	if e.IsNonNil(err) {
		return err.PushStack()
	}

	hashe = hashe.WithParseMode(true)
	if err := hashe.EmitEditMessage(constants.OutgoingRoutingKey, msg); e.IsNonNil(err) {
		return err.PushStack()
	}
	return hashe.EmitCallback(constants.OutgoingRoutingKey, update.Callback, helpers.AnswerCallbackBanner("", update.Callback))
}
