package exportchat

import (
	"fmt"
	"strings"
	"time"

	shared "github.com/ChatDetectiveORG/command-handler/src/application/endpoints"
	"github.com/ChatDetectiveORG/command-handler/src/infrastructure/postgresql"
	cdredis "github.com/ChatDetectiveORG/command-handler/src/infrastructure/redis"
	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	models "github.com/ChatDetectiveORG/shared/postgresModels"
	"github.com/ChatDetectiveORG/shared/utils"

	tele "gopkg.in/telebot.v4"
)

// restoreCallbackMeta is the payload stashed in Redis when the user opens a chat preview, so the
// "Восстановить" button can carry a stable short id instead of trying to fit chat data inline.
type restoreCallbackMeta struct {
	InterlocutorCode string `json:"interlocutor_code"`
}

func NewViewChatEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"view_chat",
		*h.HandlerChain{}.Init(
			30*time.Second,
			h.InitChainHandler(runViewChat, h.EndOnError),
		),
		h.CallbackStartsWith(shared.UniqueGoToChat),
	)
	return ep
}

func buildMessage(callbackData string, messageID int, chatID int64, sender *models.Telegramuser) (*tele.Message, *e.ErrorInfo) {
	db := postgresql.GetDB()

	parsedData := utils.ParseCallbackData(callbackData)
	code := parsedData[shared.CallbackFieldCode]
	if code == "" {
		return nil, e.NewError("missing interlocutor code", "callback data has no chat code").WithSeverity(e.Notice)
	}

	interlocutor := &models.Telegramuser{}
	if eRaw := db.Model(interlocutor).Where("referral_code = ?", code).Select(); e.IsNonNil(eRaw) {
		return nil, e.FromError(eRaw, "failed to load interlocutor by referral code").WithSeverity(e.Notice)
	}

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

	sb := strings.Builder{}
	sb.WriteString(fmt.Sprintf("*Чат с ![%s](tg://user?id=%d)*", fullName, tgID))
	sb.WriteString("\n")
	if count > 0 {
		sb.WriteString(fmt.Sprintf("_Возможно восстановить %d сообщений_", count))
	} else {
		sb.WriteString("_К сожалению, чат не может быть восстановлен_\\. ![Почему?](https://t.me/chatdetective_support/1210)")
	}

	keyboardRow := []tele.InlineButton{
		{Text: "Назад", Data: shared.UniqueChatSelectPage},
	}
	if count > 0 {
		metaID, err := cdredis.StoreCallbackMeta(restoreCallbackMeta{InterlocutorCode: code})
		if e.IsNonNil(err) {
			return nil, err.PushStack()
		}
		keyboardRow = append(keyboardRow, tele.InlineButton{
			Text: "Восстановить",
			Data: shared.UniqueRestoreChat + "\n" + metaID,
		})
	}

	return &tele.Message{
		ID:          messageID,
		Chat:        &tele.Chat{ID: chatID},
		Text:        sb.String(),
		ReplyMarkup: &tele.ReplyMarkup{InlineKeyboard: [][]tele.InlineButton{keyboardRow}},
	}, e.Nil()
}

func runViewChat(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	db := postgresql.GetDB()
	sender, err := shared.GetUserByTgID(db, update.Callback.Sender.ID)
	if e.IsNonNil(err) {
		return err.PushStack()
	}

	msg, err := buildMessage(update.Callback.Data, update.Callback.Message.ID, update.Callback.Message.Chat.ID, sender)
	if e.IsNonNil(err) {
		return err.PushStack()
	}

	hashe = hashe.WithParseMode(true)
	if err := hashe.EmitEditMessage(shared.OutgoingRoutingKey, msg); e.IsNonNil(err) {
		return err.PushStack()
	}
	return hashe.EmitCallback(shared.OutgoingRoutingKey, update.Callback, shared.AnswerCallbackBanner("", update.Callback))
}
