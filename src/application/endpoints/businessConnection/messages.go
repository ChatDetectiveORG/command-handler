package businessconnection

import (
	"strings"

	shared "github.com/ChatDetectiveORG/command-handler/src/application/endpoints"
	"github.com/ChatDetectiveORG/command-handler/src/infrastructure/postgresql"
	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	models "github.com/ChatDetectiveORG/shared/postgresModels"
	utils "github.com/ChatDetectiveORG/shared/utils"
	tele "gopkg.in/telebot.v4"
)

func buildConnectedMessage(chatID int64) *tele.Message {
	return &tele.Message{
		Chat: &tele.Chat{ID: chatID},
		Text: "Бот подключен, все работает как надо!👌\n\nТеперь:\n👍Ты будешь получать уведомления, если кто-то удалит или изменит сообщения в личных чатах \n👍Ты сможешь скачивать фото, видео, голосовые сообщения и кружочки которые обычно исчезают после одного просмотра\n👍У тебя будет возможность восстановить чат даже после его удаления \n\nВ общем, полный контроль над собеседником!",
		Entities: tele.Entities{
			{Type: tele.EntityBold, Offset: 0, Length: 37},
			{Type: tele.EntityBold, Offset: 37, Length: 2},
			{Type: tele.EntityCustomEmoji, Offset: 37, Length: 2, CustomEmojiID: "5463423955014529788"},
			{Type: tele.EntityBold, Offset: 39, Length: 1},
			{Type: tele.EntityCustomEmoji, Offset: 49, Length: 2, CustomEmojiID: "5465465194056525619"},
			{Type: tele.EntityCustomEmoji, Offset: 140, Length: 2, CustomEmojiID: "5465465194056525619"},
			{Type: tele.EntityCustomEmoji, Offset: 254, Length: 2, CustomEmojiID: "5465465194056525619"},
		},
	}
}

func buildDisconnectedMessage(chatID int64) *tele.Message {
	return &tele.Message{
		Chat: &tele.Chat{ID: chatID},
		Text: "Бот отключён!🙈\n\nТеперь большая часть функций недоступна. Бот будет работать только в тех чатах, где собеседник использует " + shared.BotUsername,
		Entities: tele.Entities{
			{Type: tele.EntityBold, Offset: 0, Length: 13},
			{Type: tele.EntityBold, Offset: 13, Length: 2},
			{Type: tele.EntityCustomEmoji, Offset: 13, Length: 2, CustomEmojiID: "5463345378587849154"},
			{Type: tele.EntityBold, Offset: 15, Length: 1},
			{Type: tele.EntityMention, Offset: 123, Length: utils.TgLen(shared.BotUsername)},
		},
		ReplyMarkup: &tele.ReplyMarkup{
			InlineKeyboard: [][]tele.InlineButton{
				{{Text: "Показать список", Data: shared.UniqueShowContacts}},
			},
		},
	}
}

func buildRelationConnectedMessage(connectedUser *models.Telegramuser, chatID int64) (*tele.Message, *e.ErrorInfo) {
	fullName, err := connectedUser.GetFullName()
	if e.IsNonNil(err) {
		return nil, err
	}
	fullName = strings.TrimSpace(fullName)
	tgID, err := connectedUser.GetTgId()
	if e.IsNonNil(err) {
		return nil, err
	}
	nameLen := utils.TgLen(fullName)
	botMentionOffset := nameLen + utils.TgLen(" теперь использует ")
	botMentionLen := utils.TgLen(shared.BotUsername)
	thermOffset := botMentionOffset + botMentionLen
	thumbsOffset := thermOffset + utils.TgLen("!🌡\n\nТеперь:\n")
	secondThumbsOffset := thumbsOffset + utils.TgLen("👍Ты будешь получать уведомления, если он удалит или изменит сообщения в личном чате\n")
	thirdThumbsOffset := secondThumbsOffset + utils.TgLen("👍Ты сможешь скачивать фото, видео, голосовые сообщения и кружочки которые обычно исчезают после одного просмотра\n")

	text := fullName + " теперь использует " + shared.BotUsername + "!🌡\n\nТеперь:\n" +
		"👍Ты будешь получать уведомления, если он удалит или изменит сообщения в личном чате\n" +
		"👍Ты сможешь скачивать фото, видео, голосовые сообщения и кружочки которые обычно исчезают после одного просмотра\n" +
		"👍У тебя будет возможность восстановить чат с ним даже после его удаления"

	entities := tele.Entities{
		{Type: tele.EntityTMention, Offset: 0, Length: nameLen, User: &tele.User{ID: tgID}},
		{Type: tele.EntityBold, Offset: 0, Length: nameLen},
		{Type: tele.EntityBold, Offset: nameLen, Length: utils.TgLen(" теперь использует ")},
		{Type: tele.EntityMention, Offset: botMentionOffset, Length: botMentionLen},
		{Type: tele.EntityBold, Offset: botMentionOffset, Length: botMentionLen},
		{Type: tele.EntityBold, Offset: thermOffset, Length: 1},
		{Type: tele.EntityBold, Offset: thermOffset + 1, Length: 2},
		{Type: tele.EntityCustomEmoji, Offset: thermOffset + 1, Length: 2, CustomEmojiID: "5463054218459884779"},
		{Type: tele.EntityBold, Offset: thermOffset + 3, Length: 2},
		{Type: tele.EntityCustomEmoji, Offset: thumbsOffset, Length: 2, CustomEmojiID: "5465465194056525619"},
		{Type: tele.EntityCustomEmoji, Offset: secondThumbsOffset, Length: 2, CustomEmojiID: "5465465194056525619"},
		{Type: tele.EntityCustomEmoji, Offset: thirdThumbsOffset, Length: 2, CustomEmojiID: "5465465194056525619"},
	}

	return &tele.Message{
		Chat:     &tele.Chat{ID: chatID},
		Text:     text,
		Entities: entities,
	}, e.Nil()
}

func buildRelationDisconnectedMessage(disconnectedUser *models.Telegramuser, chatID int64) (*tele.Message, *e.ErrorInfo) {
	fullName, err := disconnectedUser.GetFullName()
	if e.IsNonNil(err) {
		return nil, err
	}
	fullName = strings.TrimSpace(fullName)
	tgID, err := disconnectedUser.GetTgId()
	if e.IsNonNil(err) {
		return nil, err
	}
	nameLen := utils.TgLen(fullName)
	botMentionOffset := nameLen + utils.TgLen(" отключил ")
	botMentionLen := utils.TgLen(shared.BotUsername)
	emojiOffset := botMentionOffset + botMentionLen
	secondMentionOffset := utils.TgLen(fullName+" отключил "+shared.BotUsername+"!🙈\n\nТеперь большая часть функций в чате с ним недоступна. Бот будет работать только в тех чатах, где собеседник использует ")

	text := fullName + " отключил " + shared.BotUsername + "!🙈\n\nТеперь большая часть функций в чате с ним недоступна. Бот будет работать только в тех чатах, где собеседник использует " + shared.BotUsername

	entities := tele.Entities{
		{Type: tele.EntityTMention, Offset: 0, Length: nameLen, User: &tele.User{ID: tgID}},
		{Type: tele.EntityBold, Offset: 0, Length: nameLen},
		{Type: tele.EntityBold, Offset: nameLen, Length: utils.TgLen(" отключил ")},
		{Type: tele.EntityMention, Offset: botMentionOffset, Length: botMentionLen},
		{Type: tele.EntityBold, Offset: botMentionOffset, Length: botMentionLen},
		{Type: tele.EntityBold, Offset: emojiOffset, Length: 1},
		{Type: tele.EntityBold, Offset: emojiOffset + 1, Length: 2},
		{Type: tele.EntityCustomEmoji, Offset: emojiOffset + 1, Length: 2, CustomEmojiID: "5463345378587849154"},
		{Type: tele.EntityBold, Offset: emojiOffset + 3, Length: 2},
		{Type: tele.EntityMention, Offset: secondMentionOffset, Length: botMentionLen},
	}

	return &tele.Message{
		Chat:     &tele.Chat{ID: chatID},
		Text:     text,
		Entities: entities,
	}, e.Nil()
}

func run(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	db := postgresql.GetDB()
	conn := update.BusinessConnection
	userChatID := conn.UserChatID

	user, err := shared.GetUserByTgID(db, conn.Sender.ID)
	if e.IsNonNil(err) {
		return err
	}

	if conn.Enabled {
		if err := shared.UpdateBusinessConnectionIDHash(db, user, conn.ID); e.IsNonNil(err) {
			return err
		}

		if err := hashe.Emit(shared.OutgoingRoutingKey, buildConnectedMessage(userChatID)); e.IsNonNil(err) {
			return err
		}

		nonBotUsers, err := shared.UserRelatedNonBotUsers(db, user)
		if e.IsNonNil(err) {
			return err
		}

		for _, relatedUser := range nonBotUsers {
			relatedTgID, tgErr := relatedUser.GetTgId()
			if e.IsNonNil(tgErr) {
				continue
			}
			notifyMsg, buildErr := buildRelationConnectedMessage(user, relatedTgID)
			if e.IsNonNil(buildErr) {
				continue
			}
			_ = hashe.Emit(shared.OutgoingRoutingKey, notifyMsg)
		}
	} else {
		if err := shared.UpdateBusinessConnectionIDHash(db, user, ""); e.IsNonNil(err) {
			return err
		}

		disconnectedMsg := buildDisconnectedMessage(userChatID)
		if err := hashe.Emit(shared.OutgoingRoutingKey, disconnectedMsg); e.IsNonNil(err) {
			return err
		}

		nonBotUsers, err := shared.UserRelatedNonBotUsers(db, user)
		if e.IsNonNil(err) {
			return err
		}

		for _, relatedUser := range nonBotUsers {
			relatedTgID, tgErr := relatedUser.GetTgId()
			if e.IsNonNil(tgErr) {
				continue
			}
			notifyMsg, buildErr := buildRelationDisconnectedMessage(user, relatedTgID)
			if e.IsNonNil(buildErr) {
				continue
			}
			_ = hashe.Emit(shared.OutgoingRoutingKey, notifyMsg)
		}
	}

	return e.Nil()
}
