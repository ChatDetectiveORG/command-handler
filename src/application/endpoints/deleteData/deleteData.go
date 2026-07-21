package deletedata

import (
	"time"

	"github.com/ChatDetectiveORG/command-handler/src/infrastructure/postgresql"
	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	telegram "github.com/ChatDetectiveORG/shared/messageBuilder"
	models "github.com/ChatDetectiveORG/shared/postgresModels"
	"github.com/go-pg/pg/v10"
	tele "gopkg.in/telebot.v4"

	helpers "github.com/ChatDetectiveORG/shared/commandHandlerUtils"
	constants "github.com/ChatDetectiveORG/shared/constants"

	. "github.com/ChatDetectiveORG/shared/messageBuilder"
)

func NewDeleteDataEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"delete_data",
		*h.HandlerChain{}.Init(
			30*time.Second,
			h.InitChainHandler(runDeleteData, h.EndOnError),
		),
		h.Or(h.Command([]string{"delete_data"}), h.TextCommand("Удалить данные")),
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
		h.UniqueCallback(constants.UniqueDeleteConfirm),
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
		h.UniqueCallback(constants.UniqueDeleteCancel),
	)
	return ep
}

// Builds data deletion warning message
func buildWarningMessage(chatID int64) *tele.Message {
	messageBuilder := telegram.MessageBuilder{Mdv2Enabled: true}
	messageBuilder.WriteString(
		"⚠️", telegram.TextFormat{Type: telegram.FormatLink}.WithCustomEmojiID("5395358455768837479"),
	).WriteString(
		"ВНИМАНИЕ", telegram.TextFormat{Type: telegram.FormatBold},
	).WriteString(
		"⚠️", telegram.TextFormat{Type: telegram.FormatLink}.WithCustomEmojiID("5395358455768837479"),
	).WriteString(
		"\nУдаление данных сотрёт всю информацию о вас с наших серверов, включая совершённые транзакции. Это значит, что вы ",
	).WriteString(
		"НЕ", telegram.TextFormat{Type: telegram.FormatBold},
	).WriteString(
		" сможете восстановить свои чаты в случае их удаления и совершённые покупки.\n\n",
	).WriteString(
		"Это действие нельзя отменить. Вы уверены, что хотите стереть все данные?", telegram.TextFormat{Type: telegram.FormatItalic},
	)

	messageBuilder.AddButton(tele.InlineButton{Text: "Нет!", Data: constants.UniqueDeleteCancel})
	messageBuilder.AddButton(tele.InlineButton{Text: "Да.", Data: constants.UniqueDeleteConfirm})

	return messageBuilder.Build(chatID)
}

func runDeleteData(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	return hashe.WithParseMode(true).Emit(constants.OutgoingRoutingKey, buildWarningMessage(update.Message.Chat.ID))
}

// Sends message about data deletion cancellation
func runDeleteCancel(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	if err := hashe.EmitCallback(
		constants.OutgoingRoutingKey,
		update.Callback,
		helpers.AnswerCallbackBanner("Данные не будут удалены", update.Callback),
	); e.IsNonNil(err) {
		return err
	}

	deleteMsg := &tele.Message{
		ID:   update.Callback.Message.ID,
		Chat: update.Callback.Message.Chat,
	}
	return hashe.EmitDeleteMessage(constants.OutgoingRoutingKey, deleteMsg)
}

// Deletes ALL user data
//
// Supported models: UserSettings, UserLevels, Messages, MessageVersions, Mirrors, Payments, Referrals, Telegramuser
//
// UserRelations are not deleted, because deletion can lead to poor experience of interlocutors in chats with user that initiated data deletion. (they are owners of this data too.)
func runDeleteConfirm(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	// answerCallbackQuery must reach Telegram quickly; DB work and deleteMessage are slower.
	if err := hashe.EmitCallback(
		constants.OutgoingRoutingKey,
		update.Callback,
		helpers.AnswerCallbackBanner("Удаление данных...", update.Callback),
	); e.IsNonNil(err) {
		return err
	}

	db := postgresql.GetDB()
	tgUserID := update.Callback.Sender.ID

	user, err := helpers.GetUserByTgID(db, tgUserID)
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

	b := MessageBuilder{}
	b.Write(T("Данные удалены успешно!"))

	err = hashe.Emit(constants.OutgoingRoutingKey, b.Build(update.Callback.Message.Chat.ID))
	if e.IsNonNil(err) {
		return err
	}

	deleteMsg := &tele.Message{
		ID:   update.Callback.Message.ID,
		Chat: update.Callback.Message.Chat,
	}
	return hashe.EmitDeleteMessage(constants.OutgoingRoutingKey, deleteMsg)
}
