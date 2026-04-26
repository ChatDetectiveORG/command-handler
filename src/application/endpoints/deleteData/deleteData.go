package deletedata

import (
	"time"

	shared "github.com/ChatDetectiveORG/command-handler/src/application/endpoints"
	"github.com/ChatDetectiveORG/command-handler/src/infrastructure/postgresql"
	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	models "github.com/ChatDetectiveORG/shared/postgresModels"
	"github.com/go-pg/pg/v10"
	tele "gopkg.in/telebot.v4"
)

func NewDeleteDataEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"delete_data",
		*h.HandlerChain{}.Init(
			30*time.Second,
			h.InitChainHandler(runDeleteData, h.EndOnError),
		),
		h.Command([]string{"deleteData"}),
	)
	return ep
}

func NewDeleteConfirmEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"delete_confirm",
		*h.HandlerChain{}.Init(
			2*time.Minute,
			h.InitChainHandler(runDeleteConfirm, h.EndOnError),
		),
		h.UniqueCallback(shared.UniqueDeleteConfirm),
	)
	return ep
}

func NewDeleteCancelEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"delete_cancel",
		*h.HandlerChain{}.Init(
			30*time.Second,
			h.InitChainHandler(runDeleteCancel, h.EndOnError),
		),
		h.UniqueCallback(shared.UniqueDeleteCancel),
	)
	return ep
}

func buildWarningMessage(chatID int64) *tele.Message {
	return &tele.Message{
		Chat: &tele.Chat{ID: chatID},
		Text: "⚠️ВНИМАНИЕ⚠️\nУдаление данных сотрёт всю информацию о вас с наших серверов, включая совершённые транзакции. Это значит, что вы НЕ сможете восстановить свои чаты в случае их удаления и совершённые покупки.\n\nЭто действие нельзя отменить. Вы уверены, что хотите стереть все данные?",
		Entities: tele.Entities{
			{Type: tele.EntityCustomEmoji, Offset: 0, Length: 2, CustomEmojiID: "5395358455768837479"},
			{Type: tele.EntityBold, Offset: 2, Length: 8},
			{Type: tele.EntityCustomEmoji, Offset: 10, Length: 2, CustomEmojiID: "5395358455768837479"},
			{Type: tele.EntityBold, Offset: 126, Length: 2},
			{Type: tele.EntityItalic, Offset: 235, Length: 42},
		},
		ReplyMarkup: &tele.ReplyMarkup{
			InlineKeyboard: [][]tele.InlineButton{
				{
					{Text: "Нет!", Data: shared.UniqueDeleteCancel},
					{Text: "Да.", Data: shared.UniqueDeleteConfirm},
				},
			},
		},
	}
}

func runDeleteData(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	return hashe.Emit(shared.OutgoingRoutingKey, buildWarningMessage(update.Message.Chat.ID))
}

func runDeleteCancel(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	deleteMsg := &tele.Message{
		ID:   update.Callback.Message.ID,
		Chat: update.Callback.Message.Chat,
	}
	if err := hashe.EmitDeleteMessage(shared.OutgoingRoutingKey, deleteMsg); e.IsNonNil(err) {
		return err
	}
	return hashe.EmitCallback(
		shared.OutgoingRoutingKey,
		update.Callback,
		shared.AnswerCallbackBanner("Данные не будут удалены", update.Callback),
	)
}

func runDeleteConfirm(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	db := postgresql.GetDB()
	tgUserID := update.Callback.Sender.ID

	user, err := shared.GetUserByTgID(db, tgUserID)
	if e.IsNonNil(err) {
		return err
	}

	tx, eraw := db.Begin()
	if eraw != nil {
		return e.FromError(eraw, "failed to begin delete transaction").WithSeverity(e.Critical)
	}
	defer tx.Rollback()

	// Delete MessageVersions then Messages for this user's business connection.
	if user.BusinessConnectionIDHash != "" {
		var messagesToDelete []models.Message
		if eraw = tx.Model(&messagesToDelete).
			Where("business_connection_id_hash = ?", user.BusinessConnectionIDHash).
			Column("id").
			Select(); eraw == nil && len(messagesToDelete) > 0 {

			msgIDs := make([]int, 0, len(messagesToDelete))
			for _, m := range messagesToDelete {
				msgIDs = append(msgIDs, m.ID)
			}
			_, _ = tx.Model((*models.MessageVersion)(nil)).
				Where("message_id IN (?)", pg.In(msgIDs)).
				Delete()
		}

		_, _ = tx.Model((*models.Message)(nil)).
			Where("business_connection_id_hash = ?", user.BusinessConnectionIDHash).
			Delete()
	}

	// Delete UserRelations (both directions).
	_, _ = tx.Model((*models.UserRelations)(nil)).
		Where("first_user_id = ?", user.ID).Delete()
	_, _ = tx.Model((*models.UserRelations)(nil)).
		Where("second_user_id = ?", user.ID).Delete()

	// Delete UserLevels.
	_, _ = tx.Model((*models.UserLevels)(nil)).
		Where("linked_user_id = ?", user.ID).
		Delete()

	// Delete UserSettings.
	_, _ = tx.Model((*models.UserSettings)(nil)).
		Where("linked_user_id = ?", user.ID).
		Delete()

	// Delete the user.
	_, _ = tx.Model(user).WherePK().Delete()

	if eraw = tx.Commit(); eraw != nil {
		return e.FromError(eraw, "failed to commit delete transaction").WithSeverity(e.Critical)
	}

	deleteMsg := &tele.Message{
		ID:   update.Callback.Message.ID,
		Chat: update.Callback.Message.Chat,
	}
	_ = hashe.EmitDeleteMessage(shared.OutgoingRoutingKey, deleteMsg)

	return hashe.EmitCallback(
		shared.OutgoingRoutingKey,
		update.Callback,
		shared.AnswerCallbackBanner("Данные удалены успешно!", update.Callback),
	)
}
