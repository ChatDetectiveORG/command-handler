package businessconnection

import (
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	shared "github.com/ChatDetectiveORG/command-handler/src/application/endpoints"
	"github.com/ChatDetectiveORG/command-handler/src/infrastructure/postgresql"
	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	models "github.com/ChatDetectiveORG/shared/postgresModels"
	utils "github.com/ChatDetectiveORG/shared/utils"
	"github.com/go-pg/pg/v10"
	tele "gopkg.in/telebot.v4"
)

func getReferralAndLinkOwner(tx *pg.Tx, actor *models.Telegramuser) (*models.Referral, *models.Telegramuser, *e.ErrorInfo) {
	referral := &models.Referral{}
	err := e.Wrap(tx.Model(referral).Where("invited_user_id = ?", actor.ID).Select())
	if e.IsNonNil(err) {
		if err.Err.Error() != "no rows in result set" {
			return nil, nil, err.WithSeverity(e.Ingnored)
		}

		return nil, nil, err
	}

	invitor := &models.Telegramuser{
		ID: referral.InvitorID,
	}
	err = e.Wrap(tx.Model(invitor).Where("id = ?", invitor.ID).Relation("Settings").Select())
	if e.IsNonNil(err) {
		return nil, nil, err
	}

	return referral, invitor, e.Nil()
}

func updateReferral(db *pg.DB, actor *models.Telegramuser, connected bool) *e.ErrorInfo {
	tx, eraw := db.Begin()
	if eraw != nil {
		return e.FromError(eraw, "failed to begin transaction").WithSeverity(e.Critical)
	}
	defer tx.Rollback()

	referral, invitor, err := getReferralAndLinkOwner(tx, actor)
	if e.IsNonNil(err) {
		return err
	}

	if invitor.Settings == nil {
		return e.NewError("user_settings_not_found", "user settings not found").WithSeverity(e.Ingnored)
	}

	referral.FixedRewardType = invitor.Settings.ReferralBonusPreference

	switch invitor.Settings.ReferralBonusPreference {
	case models.ReferralBonusMoney:
		referral.FixedMoneyReward = shared.ReferralBonusRub
	case models.ReferralBonusLevels:
		referral.ActualUntil = time.Now().Add(time.Duration(shared.ReferralLevelsDurationSec) * time.Second)
	}

	referral.UpdatedAt = time.Now()

	if !connected {
		referral.ActualUntil = time.Time{}
		referral.FixedMoneyReward = 0
		referral.FixedRewardType = ""
	}

	_, eRaw := tx.Model(referral).Update()
	if eRaw != nil {
		return e.FromError(eRaw, "failed to update referral").WithSeverity(e.Critical)
	}

	return e.Nil()
}

func recountLevels(tx *pg.Tx, untrackedRalations []models.Referral, invitedUserID []byte) (int, *e.ErrorInfo) {
	var levelsAdded int

	for i := 0; i < len(untrackedRalations); i+=shared.ReferralBonusThresholdLevels {
		var addedRelationsDurations []int64
		var addedRelationsIDs []int

		for j := i; j < i+shared.ReferralBonusThresholdLevels; j++ {
			addedRelationsDurations = append(addedRelationsDurations, untrackedRalations[j].ActualUntil.Unix())
			addedRelationsIDs = append(addedRelationsIDs, untrackedRalations[j].ID)
		}
		
		newLevel := models.UserLevels{
			LinkedUserID: invitedUserID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Level: shared.ReferralBonusThresholdLevels,
			UntilTimestamp: time.Now().Add(time.Duration(slices.Max(addedRelationsDurations)) * time.Second).Unix(),
			IsReferralBonus: true,
			LinkedReferralIDs: addedRelationsIDs,
		}

		_, eRaw := tx.Model(&newLevel).Insert()
		if e.IsNonNil(eRaw) {
			return levelsAdded, e.Wrap(eRaw)
		}

		levelsAdded += 1
	}

	return levelsAdded, e.Nil()
}

func handleLevels(tx *pg.Tx, referral *models.Referral, sb *strings.Builder, connected bool) *e.ErrorInfo {
	err := e.Wrap(tx.Model(&models.UserLevels{}).
		Where("? = ANY(linked_referral_ids)", referral.ID).
		Select(),
	)
	if e.IsNil(err) && connected {
		return e.NewError("referral already considered", "referral already considered").WithSeverity(e.Ingnored)
	}

	var untrackedRalations []models.Referral
	err = e.Wrap(tx.Model(&untrackedRalations).
		Where("id NOT IN (SELECT unnest(linked_referral_ids) FROM user_levels WHERE linked_user_id = ?)", referral.InvitedUserID).
		Order("active_until ASC").
		Select(),
	)

	levelSummary, err := models.GetUserLevelSummary(tx, referral.InvitedUserID, time.Now())
	if e.IsNonNil(err) {
		return err
	}

	if !connected {
		var addictedBonus models.UserLevels
		err = e.Wrap(tx.Model(&addictedBonus).
			Where("? = ANY(linked_referral_ids)", referral.ID).
			Limit(1).
			Select(),
		)
		if e.IsNonNil(err) {
			return err
		}

		_, eRaw := tx.Model(&addictedBonus).WherePK().Delete()
		if e.IsNonNil(eRaw) {
			return e.Wrap(eRaw)
		}

		levelsAdded := 0
		if len(addictedBonus.LinkedReferralIDs) != shared.ReferralBonusThresholdLevels {
			levelsAdded, err = recountLevels(tx, untrackedRalations, referral.InvitedUserID)
			if e.IsNonNil(err) {
				return err
			}
		}

		if levelsAdded == 0 {
			sb.WriteString("![🚫](tg://emoji?id=5462882007451185227)Ваш уровень снижен на 1.\n")	
		} else {
			sb.WriteString("Твой уровень сейчас составляет ")
			sb.WriteString(strconv.Itoa(levelSummary.Level) + "\n\n")
		}

		sb.WriteString("Чтобы повысить его, приведи ещё " + strconv.Itoa(shared.ReferralBonusThresholdLevels - 1) + " пользователей\n\n")

		return e.Nil()		
	}

	if len(untrackedRalations) < shared.ReferralBonusThresholdLevels {
		sb.WriteString("Твой уровень сейчас составляет ")
		sb.WriteString(strconv.Itoa(levelSummary.Level) + "\n\n")
		sb.WriteString("Для получения бонусного уровня нужно пригласить ещё ")
		sb.WriteString(strconv.Itoa(shared.ReferralBonusThresholdLevels - len(untrackedRalations)) + " пользователей\n\n")

		return e.Nil()
	}

	levelsAdded, err := recountLevels(tx, untrackedRalations, referral.InvitedUserID)
	if e.IsNonNil(err) {
		return err
	}

	sb.WriteString("Твой уровень сейчас составляет ")
	sb.WriteString(strconv.Itoa(levelSummary.Level + levelsAdded) + "\n\n")
	sb.WriteString("Ты получил " + strconv.Itoa(levelsAdded) + " уровней\n")
	sb.WriteString("Приведи " + strconv.Itoa(shared.ReferralBonusThresholdLevels - (len(untrackedRalations) - levelsAdded * shared.ReferralBonusThresholdLevels)) + " пользователей чтобы получить ещё один бонусный уровень!")

	return e.Nil()
}

func handleMoney(sb *strings.Builder, connected bool) {
	if connected {
		sb.WriteString("![🔥](tg://emoji?id=5256047523620995497)Если он не будет отключать бота до ")
		sb.WriteString(time.Now().Add(time.Duration(shared.ReferralDiscountDurationSec) * time.Second + time.Hour * 24).Format("02.01.2006"))
		sb.WriteString(" ты получишь " + strconv.FormatInt(shared.ReferralBonusRub, 10) + " рублей на внутренний счёт бота")

		return 
	}

	sb.WriteString("![🚫](tg://emoji?id=5462882007451185227)Ты не получишь выплат за него\n")
	sb.WriteString("Чтобы получить денги за него, он должен подключить бота и пользоваться им без перерыва в течение ")
	sb.WriteString(strconv.FormatFloat(math.Ceil(float64(time.Duration(shared.ReferralDiscountDurationSec).Hours() / 24)), 'f', -1, 64) + " дней")
}

func sendReferral(db *pg.DB, actor *models.Telegramuser, hashe *h.HandlerChainHashe, connected bool) *e.ErrorInfo {
	tx, eraw := db.Begin()
	if eraw != nil {
		return e.FromError(eraw, "failed to begin transaction").WithSeverity(e.Critical)
	}
	defer tx.Rollback()

	referral, invitor, err := getReferralAndLinkOwner(tx, actor)
	if e.IsNonNil(err) {
		return err
	}

	sb := strings.Builder{}
	sb.WriteString("![🔝](tg://emoji?id=5463071033256848094)Пользователь ")
	actorFullName, err := actor.GetFullName()
	if e.IsNonNil(err) {
		return err
	}
	actorTgID, err := actor.GetTgId()
	if e.IsNonNil(err) {
		return err
	}
	sb.WriteString("![" + utils.EscapeMarkdownV2(actorFullName) + "](tg://user?id=" + strconv.FormatInt(actorTgID, 10) + ")")
	if connected {
		sb.WriteString(" подключил бота!\n\n")
	} else {
		sb.WriteString(" отключил бота!\n\n")
	}

	keyboard := [][]tele.InlineButton{}

	switch invitor.Settings.ReferralBonusPreference {
	case models.ReferralBonusMoney:
		handleMoney(&sb, connected)
		
		keyboard = append(keyboard, []tele.InlineButton{
			{Text: "Посмотреть баланс"},
			{Text: "Подробнее"},
		})
	case models.ReferralBonusLevels:
		err = handleLevels(tx, referral, &sb, connected)
		if e.IsNonNil(err) {
			return err
		}

		keyboard = append(keyboard, []tele.InlineButton{
			{Text: "Посмотреть уровни"},
		})
	}

	message := &tele.Message{
		Chat: &tele.Chat{ID: actorTgID},
		Text: sb.String(),
		ReplyMarkup: &tele.ReplyMarkup{
			InlineKeyboard: keyboard,
		},
	}

	return hashe.Emit(shared.OutgoingRoutingKey, message)
}

func referralMain(db *pg.DB, actor *models.Telegramuser, hashe *h.HandlerChainHashe, connected bool) *e.ErrorInfo {
	err := updateReferral(db, actor, connected)
	if e.IsNonNil(err) {
		return err
	}

	return sendReferral(db, actor, hashe, connected)
}


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
	secondMentionOffset := utils.TgLen(fullName + " отключил " + shared.BotUsername + "!🙈\n\nТеперь большая часть функций в чате с ним недоступна. Бот будет работать только в тех чатах, где собеседник использует ")

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

		err = referralMain(db, user, hashe, true)
		if e.IsNonNil(err) {
			return err
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
		err = referralMain(db, user, hashe, false)
		if e.IsNonNil(err) {
			return err
		}
	}

	return e.Nil()
}
