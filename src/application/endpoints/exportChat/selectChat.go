package exportchat

import (
	"fmt"
	"math"
	"strconv"
	"time"

	shared "github.com/ChatDetectiveORG/command-handler/src/application/endpoints"
	"github.com/ChatDetectiveORG/command-handler/src/infrastructure/postgresql"
	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	models "github.com/ChatDetectiveORG/shared/postgresModels"
	"github.com/ChatDetectiveORG/shared/utils"
	"github.com/go-pg/pg/v10"
	tele "gopkg.in/telebot.v4"
)

// pageSize is the number of contacts shown per pagination page.
const pageSize = 7

func NewSelectChatEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"show_contacts",
		*h.HandlerChain{}.Init(
			30*time.Second,
			h.InitChainHandler(runShowContacts, h.EndOnError),
		),
		h.Or(h.Command([]string{"export"}), h.TextCommand("Экспорт чата"), h.CallbackStartsWith(shared.UniqueChatSelectPage)),
	)
	return ep
}

func selectUsers(user *models.Telegramuser, callbackData string, db *pg.DB) ([]*models.Telegramuser, int, *e.ErrorInfo) {
	parsedData := utils.ParseCallbackData(callbackData)
	page := parsedData[shared.CallbackFieldPage]
	if page == "" {
		page = "0"
	}
	pageInt, eRaw := strconv.Atoi(page)
	if eRaw != nil {
		return nil, 0, e.FromError(eRaw, "failed to convert page to int").WithSeverity(e.Notice)
	}

	count, eRaw := db.Model((*models.Telegramuser)(nil)).
		Join("JOIN user_relations AS r ON (r.first_user_id = telegramuser.id AND r.second_user_id = ?) OR (r.second_user_id = telegramuser.id AND r.first_user_id = ?)", user.ID, user.ID).
		Where("telegramuser.id != ?", user.ID).
		Count()
	if e.IsNonNil(eRaw) {
		return nil, 0, e.FromError(eRaw, "failed to get count").WithSeverity(e.Notice)
	}

	maxPage := int(math.Ceil(float64(count)/float64(pageSize))) - 1
	if maxPage < 0 {
		maxPage = 0
	}
	if pageInt > maxPage {
		pageInt = 0
	}
	if pageInt < 0 {
		pageInt = maxPage
	}

	var users []*models.Telegramuser
	eRaw = db.Model(&users).
		Join("JOIN user_relations AS r ON (r.first_user_id = telegramuser.id AND r.second_user_id = ?) OR (r.second_user_id = telegramuser.id AND r.first_user_id = ?)", user.ID, user.ID).
		Where("telegramuser.id != ?", user.ID).
		Order("telegramuser.created_at ASC").
		Limit(pageSize).
		Offset(pageInt * pageSize).
		Select()
	if e.IsNonNil(eRaw) {
		return nil, pageInt, e.FromError(eRaw, "failed to get users").WithSeverity(e.Notice)
	}

	return users, pageInt, e.Nil()
}

// chatMessageCount returns the number of stored messages in the private chat between user (the bot
// owner) and interlocutor. The bot saves messages with chat_id_hash = the OTHER party's tg id, so
// for a private chat that single hash is enough to identify both directions.
func chatMessageCount(db *pg.DB, user, interlocutor *models.Telegramuser) (int, *e.ErrorInfo) {
	count, eRaw := db.Model((*models.Message)(nil)).
		Where("chat_id_hash = ?", interlocutor.IDHash).
		Where("business_connection_id_hash = ?", user.BusinessConnectionIDHash).
		Where("is_deleted = false").
		Count()
	if e.IsNonNil(eRaw) {
		return 0, e.FromError(eRaw, "failed to count chat messages").WithSeverity(e.Notice)
	}
	return count, e.Nil()
}

func buildSelectChatKeyboard(user *models.Telegramuser, callbackData string) *tele.ReplyMarkup {
	db := postgresql.GetDB()

	buildBtn := func(interlocutor *models.Telegramuser) tele.InlineButton {
		fullName, err := interlocutor.GetFullName()
		if e.IsNonNil(err) {
			return tele.InlineButton{}
		}

		count, err := chatMessageCount(db, user, interlocutor)
		if e.IsNonNil(err) {
			return tele.InlineButton{}
		}

		return tele.InlineButton{
			Text: fmt.Sprintf("%s [%d]", fullName, count),
			Data: utils.DumpCallbackData(shared.UniqueGoToChat, map[string]any{shared.CallbackFieldCode: interlocutor.ReferralCode}),
		}
	}

	keyboard := [][]tele.InlineButton{}
	users, page, err := selectUsers(user, callbackData, db)
	if e.IsNonNil(err) {
		return nil
	}
	for _, u := range users {
		keyboard = append(keyboard, []tele.InlineButton{buildBtn(u)})
	}

	keyboard = append(keyboard, []tele.InlineButton{
		{Text: "<---<<", Data: utils.DumpCallbackData(shared.UniqueChatSelectPage, map[string]any{shared.CallbackFieldPage: page - 1})},
		{Text: ">>--->", Data: utils.DumpCallbackData(shared.UniqueChatSelectPage, map[string]any{shared.CallbackFieldPage: page + 1})},
	})

	return &tele.ReplyMarkup{InlineKeyboard: keyboard}
}

func buildSelectChatMessage(chatID int64, msgID int, user *models.Telegramuser, callbackData string) *tele.Message {
	return &tele.Message{
		ID:          msgID,
		Chat:        &tele.Chat{ID: chatID},
		Text:        "Выберите чат для восстановления:",
		ReplyMarkup: buildSelectChatKeyboard(user, callbackData),
	}
}

func runShowContacts(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	db := postgresql.GetDB()

	if update.Callback == nil {
		sender, err := shared.GetUserByTgID(db, update.Message.Sender.ID)
		if e.IsNonNil(err) {
			return err.PushStack()
		}

		err = hashe.Emit(shared.OutgoingRoutingKey, buildSelectChatMessage(update.Message.Chat.ID, update.Message.ID, sender, ""))
		if e.IsNonNil(err) {
			return err.PushStack()
		}

		return e.Nil()
	}

	sender, err := shared.GetUserByTgID(db, update.Callback.Sender.ID)
	if e.IsNonNil(err) {
		return err.PushStack()
	}

	err = hashe.EmitEditMessage(shared.OutgoingRoutingKey, buildSelectChatMessage(update.Callback.Message.Chat.ID, update.Callback.Message.ID, sender, update.Callback.Data))
	if e.IsNonNil(err) {
		return err.PushStack()
	}
	return hashe.EmitCallback(shared.OutgoingRoutingKey, update.Callback, shared.AnswerCallbackBanner("", update.Callback))
}
