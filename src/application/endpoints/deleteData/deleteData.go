package deletedata

import (
	"time"

	shared "github.com/ChatDetectiveORG/command-handler/src/application/endpoints"
	"github.com/ChatDetectiveORG/command-handler/src/infrastructure/postgresql"
	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	models "github.com/ChatDetectiveORG/shared/postgresModels"
	"github.com/ChatDetectiveORG/shared/telegram"
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

// Builds data deletion warning message
func buildWarningMessage(chatID int64) *tele.Message {
	messageBuilder := telegram.MessageBuilder{Mdv2Enabled: true}
	messageBuilder.WriteString(
		"⚠️", telegram.TextFormat{Type: telegram.TextFormatTypeLink}.WithCustomEmojiID("5395358455768837479"),
	).WriteString(
		"ВНИМАНИЕ", telegram.TextFormat{Type: telegram.TextFormatTypeBold},
	).WriteString(
		"⚠️", telegram.TextFormat{Type: telegram.TextFormatTypeLink}.WithCustomEmojiID("5395358455768837479"),
	).WriteString(
		"\nУдаление данных сотрёт всю информацию о вас с наших серверов, включая совершённые транзакции. Это значит, что вы ",
	).WriteString(
		"НЕ", telegram.TextFormat{Type: telegram.TextFormatTypeBold},
	).WriteString(
		" сможете восстановить свои чаты в случае их удаления и совершённые покупки.\n\n",
	).WriteString(
		"Это действие нельзя отменить. Вы уверены, что хотите стереть все данные?", telegram.TextFormat{Type: telegram.TextFormatTypeItalic},
	)

	messageBuilder.AddButton(tele.InlineButton{Text: "Нет!", Data: shared.UniqueDeleteCancel})
	messageBuilder.AddButton(tele.InlineButton{Text: "Да.", Data: shared.UniqueDeleteConfirm})

	return messageBuilder.Build(chatID)
}

func runDeleteData(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	return hashe.Emit(shared.OutgoingRoutingKey, buildWarningMessage(update.Message.Chat.ID))
}

// Sends message about data deletion cancellation
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

// Deletes ALL user data
//
// Supported models: UserSettings, UserLevels, Messages, MessageVersions, Mirrors, Payments, Referrals, Telegramuser
//
// UserRelations are not deleted, because deletion can lead to poor experience of interlocutors in chats with user that initiated data deletion. (they are owners of this data too.)
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

	// Delete UserLevels.
	_, _ = tx.Model((*models.UserLevels)(nil)).
		Where("linked_user_id = ?", user.ID).
		Delete()

	// Delete UserSettings.
	_, _ = tx.Model((*models.UserSettings)(nil)).
		Where("linked_user_id = ?", user.ID).
		Delete()

	// Delete Mirrors.
	_, _ = tx.Model((*models.Mirror)(nil)).
		Where("owner_id = ?", user.ID).
		Delete()

	// Delete Payments.
	_, _ = tx.Model((*models.Payment)(nil)).
		Where("client_id = ?", user.ID).
		Delete()
	
	// Delete referrals
	_, _ = tx.Model((*models.Referral)(nil)).
		WhereOr("invitor_id = ?", user.ID).
		WhereOr("invited_user_id = ?", user.ID).
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
