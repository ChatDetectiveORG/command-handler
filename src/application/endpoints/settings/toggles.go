package settings

import (
	shared "github.com/ChatDetectiveORG/command-handler/src/application/endpoints"
	"github.com/ChatDetectiveORG/command-handler/src/infrastructure/postgresql"
	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	models "github.com/ChatDetectiveORG/shared/postgresModels"
	tele "gopkg.in/telebot.v4"
)

// settingButton uses official icon_custom_emoji_id (custom pack emoji before label text).
func settingButton(label string, enabled bool, unique string) tele.InlineButton {
	iconID := shared.EmojiSettingOff
	if enabled {
		iconID = shared.EmojiSettingOn
	}
	return tele.InlineButton{
		Text:              label,
		Data:              unique,
		IconCustomEmojiID: iconID,
	}
}

func buildSettingsKeyboard(s *models.UserSettings) *tele.ReplyMarkup {
	return &tele.ReplyMarkup{
		InlineKeyboard: [][]tele.InlineButton{
			{settingButton("Оповещать об удалённых сообщениях", s.NotifyAboutDeletedMessages, shared.UniqueToggleDeleted)},
			{settingButton("Оповещать об изменённых сообщениях", s.NotifyAboutEditedMessages, shared.UniqueToggleEdited)},
			{settingButton("Сохранять медиа с 1 просмотром", s.SaveSelfDistructingMedia, shared.UniqueToggleSelfMedia)},
			{settingButton("Сохранять расширенную переписку*", s.ExtendedChatExport, shared.UniqueToggleExtExport)},
		},
	}
}

func buildSettingsMessage(chatID int64, msgID int, s *models.UserSettings) *tele.Message {
	return &tele.Message{
		ID:   msgID,
		Chat: &tele.Chat{ID: chatID},
		Text: "🛠НАСТРОЙКИ\nНа этой странице вы можете выбрать, о каких событиях вам будут приходить уведомления и кастомизировать бота",
		Entities: tele.Entities{
			{Type: tele.EntityCustomEmoji, Offset: 0, Length: 2, CustomEmojiID: "5462921117423384478"},
			{Type: tele.EntityBold, Offset: 2, Length: 9},
		},
		ReplyMarkup: buildSettingsKeyboard(s),
	}
}

func runShowSettings(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	db := postgresql.GetDB()
	user, settings, err := shared.GetUserByTgIDWithSettings(db, update.Message.Sender.ID)
	if e.IsNonNil(err) {
		return err
	}
	_ = user

	msg := buildSettingsMessage(update.Message.Chat.ID, 0, settings)
	return hashe.Emit(shared.OutgoingRoutingKey, msg)
}

func runToggle(update tele.Update, hashe *h.HandlerChainHashe, toggle func(*models.UserSettings)) *e.ErrorInfo {
	db := postgresql.GetDB()
	user, settings, err := shared.GetUserByTgIDWithSettings(db, update.Callback.Sender.ID)
	if e.IsNonNil(err) {
		return err
	}
	_ = user

	toggle(settings)

	_, eraw := db.Model(settings).
		WherePK().
		Column(
			"notify_about_deleted_messages",
			"notify_about_edited_messages",
			"save_self_destructing_media",
			"extended_chat_export",
		).Update()
	if eraw != nil {
		return e.FromError(eraw, "failed to update user settings").WithSeverity(e.Notice)
	}

	updatedMsg := buildSettingsMessage(
		update.Callback.Message.Chat.ID,
		update.Callback.Message.ID,
		settings,
	)
	if err := hashe.EmitEditMessage(shared.OutgoingRoutingKey, updatedMsg); e.IsNonNil(err) {
		return err
	}

	return hashe.EmitCallback(
		shared.OutgoingRoutingKey,
		update.Callback,
		shared.AnswerCallbackBanner("", update.Callback),
	)
}

func runToggleDeleted(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	return runToggle(update, hashe, func(s *models.UserSettings) {
		s.NotifyAboutDeletedMessages = !s.NotifyAboutDeletedMessages
	})
}

func runToggleEdited(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	return runToggle(update, hashe, func(s *models.UserSettings) {
		s.NotifyAboutEditedMessages = !s.NotifyAboutEditedMessages
	})
}

func runToggleSelfMedia(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	return runToggle(update, hashe, func(s *models.UserSettings) {
		s.SaveSelfDistructingMedia = !s.SaveSelfDistructingMedia
	})
}

func runToggleExtExport(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	return runToggle(update, hashe, func(s *models.UserSettings) {
		s.ExtendedChatExport = !s.ExtendedChatExport
	})
}
