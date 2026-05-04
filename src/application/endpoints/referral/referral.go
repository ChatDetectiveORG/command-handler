package referral

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	shared "github.com/ChatDetectiveORG/command-handler/src/application/endpoints"
	"github.com/ChatDetectiveORG/command-handler/src/infrastructure/postgresql"
	paymentservice "github.com/ChatDetectiveORG/payment-service"
	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	models "github.com/ChatDetectiveORG/shared/postgresModels"
	utils "github.com/ChatDetectiveORG/shared/utils"
	tele "gopkg.in/telebot.v4"
)

func NewReferralEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"referral",
		*h.HandlerChain{}.Init(
			30*time.Second,
			h.InitChainHandler(runReferral, h.EndOnError),
		),
		h.Or(
			h.Command([]string{"ref"}),
			h.TextCommand(shared.BtnInviteFriend),
		),
	)
	return ep
}

func NewBonusSelectEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"bonus_select",
		*h.HandlerChain{}.Init(
			30*time.Second,
			h.InitChainHandler(runBonusSelect, h.EndOnError),
		),
		h.UniqueCallback(shared.UniqueBonusSelect),
	)
	return ep
}

func NewBonusDetailsEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"bonus_details",
		*h.HandlerChain{}.Init(
			30*time.Second,
			h.InitChainHandler(runBonusDetails, h.EndOnError),
		),
		h.UniqueCallback(shared.UniqueBonusDetails),
	)
	return ep
}

func NewBonusBackEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"bonus_back",
		*h.HandlerChain{}.Init(
			30*time.Second,
			h.InitChainHandler(runBonusBack, h.EndOnError),
		),
		h.UniqueCallback(shared.UniqueBonusBack),
	)
	return ep
}

func NewBonusMoneyEndpoint() h.Endpoint {
	return newBonusTypeEndpoint(shared.UniqueBonusMoney, "bonus_money", models.ReferralBonusMoney)
}

func NewBonusLevelsEndpoint() h.Endpoint {
	return newBonusTypeEndpoint(shared.UniqueBonusLevels, "bonus_levels", models.ReferralBonusLevels)
}

func newBonusTypeEndpoint(unique, name, bonusType string) h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		name,
		*h.HandlerChain{}.Init(
			30*time.Second,
			h.InitChainHandler(makeBonusTypeHandler(bonusType), h.EndOnError),
		),
		h.UniqueCallback(unique),
	)
	return ep
}

func NewWhatLevelsEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"what_levels",
		*h.HandlerChain{}.Init(
			30*time.Second,
			h.InitChainHandler(runWhatLevels, h.EndOnError),
		),
		h.UniqueCallback(shared.UniqueWhatLevels),
	)
	return ep
}

func NewUpgradeLevelEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"upgrade_level",
		*h.HandlerChain{}.Init(
			3*time.Minute,
			h.InitChainHandler(runUpgradeLevel, h.EndOnError),
		),
		h.UniqueCallback(shared.UniqueUpgradeLevel),
	)
	return ep
}

func NewUpgradeLevelCommandEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"upgrade_level_command",
		*h.HandlerChain{}.Init(
			3*time.Minute,
			h.InitChainHandler(runUpgradeLevelCommand, h.EndOnError),
		),
		h.Or(
			h.Command([]string{"upgrade"}),
			h.TextCommand(shared.BtnUpgradeLevel),
		),
	)
	return ep
}

func NewLevelCommandEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"level_command",
		*h.HandlerChain{}.Init(
			30*time.Second,
			h.InitChainHandler(runLevelCommand, h.EndOnError),
		),
		h.Command([]string{"level"}),
	)
	return ep
}

// buildReferralMessage builds the main referral message for a user.
func buildReferralMessage(user *models.Telegramuser, chatID int64) *tele.Message {
	refLink := shared.BuildReferralLink(user)
	refLinkOffset := utils.TgLen("Твоя личная реферальная ссылка: ")
	refLinkLen := utils.TgLen(refLink)
	handshakeOffset := refLinkOffset + refLinkLen
	bonusText := fmt.Sprintf("%d рублей за друга (во внутренней валюте бота)", shared.ReferralBonusRub)
	bonusEmojiOffset := refLinkOffset + refLinkLen + utils.TgLen(" 🤝\nЗа приглашённых друзей ты можешь получить бонус:\n")
	crownOffset := refLinkOffset + refLinkLen + utils.TgLen(" 🤝\nЗа приглашённых друзей ты можешь получить бонус:\n"+bonusText+"🛍\nили\nРазличные бонусы в системе (скидки/бесплатные услуги на выбор)")

	text := fmt.Sprintf("Твоя личная реферальная ссылка: %s 🤝\nЗа приглашённых друзей ты можешь получить бонус:\n%d рублей за друга (во внутренней валюте бота)🛍\nили\nРазличные бонусы в системе (скидки/бесплатные услуги на выбор)👑",
		refLink,
		shared.ReferralBonusRub,
	)

	return &tele.Message{
		Chat: &tele.Chat{ID: chatID},
		Text: text,
		Entities: tele.Entities{
			{Type: tele.EntityURL, Offset: refLinkOffset, Length: refLinkLen},
			{Type: tele.EntityCustomEmoji, Offset: handshakeOffset + utils.TgLen(" "), Length: 2, CustomEmojiID: "5372957680174384345"},
			{Type: tele.EntityCustomEmoji, Offset: bonusEmojiOffset + utils.TgLen(fmt.Sprintf("%d рублей за друга (во внутренней валюте бота)", shared.ReferralBonusRub)), Length: 2, CustomEmojiID: "5453901475648390219"},
			{Type: tele.EntityCustomEmoji, Offset: crownOffset, Length: 2, CustomEmojiID: "5229011542011299168"},
		},
		PreviewOptions: &tele.PreviewOptions{Disabled: true},
		ReplyMarkup: &tele.ReplyMarkup{
			InlineKeyboard: [][]tele.InlineButton{
				{{Text: "Выбрать бонус по умолчанию", Data: shared.UniqueBonusSelect}},
				{{Text: "Подробнее", Data: shared.UniqueBonusDetails}},
			},
		},
	}
}

// buildBonusSelectionMessage builds the bonus type selection view.
func buildBonusSelectionMessage(chatID, msgID int64, currentPref string) *tele.Message {
	text := fmt.Sprintf(
		"Выберите бонус за приведённых пользователей:\n⏫%d рублей за каждого пользователя (во внутренней валюте бота)\n⏫Бесплатный уровень за каждых %d приведённых пользователей (приведённые пользователи учитываются системой 6 месяцев после подключения бота, уровни рассчитываются с округлением вниз)",
		shared.ReferralBonusRub,
		shared.ReferralBonusThresholdLevels,
	)

	arrowOffset1 := utils.TgLen("Выберите бонус за приведённых пользователей:\n")
	arrowOffset2 := arrowOffset1 + utils.TgLen(fmt.Sprintf("⏫%d рублей за каждого пользователя (во внутренней валюте бота)\n", shared.ReferralBonusRub))

	return &tele.Message{
		ID:   int(msgID),
		Chat: &tele.Chat{ID: chatID},
		Text: text,
		Entities: tele.Entities{
			{Type: tele.EntityCustomEmoji, Offset: arrowOffset1, Length: 1, CustomEmojiID: "5462995330163289902"},
			{Type: tele.EntityCustomEmoji, Offset: arrowOffset2, Length: 1, CustomEmojiID: "5462995330163289902"},
		},
		ReplyMarkup: buildBonusKeyboard(currentPref),
	}
}

func buildBonusKeyboard(currentPref string) *tele.ReplyMarkup {
	buildBtn := func(label, data string, isSelected bool) tele.InlineButton {
		btn := tele.InlineButton{Text: label, Data: data}
		if isSelected {
			btn.IconCustomEmojiID = shared.EmojiSettingOn
		}
		return btn
	}

	return &tele.ReplyMarkup{
		InlineKeyboard: [][]tele.InlineButton{
			{
				buildBtn("Деньги", shared.UniqueBonusMoney, currentPref == models.ReferralBonusMoney),
				buildBtn("Уровни", shared.UniqueBonusLevels, currentPref == models.ReferralBonusLevels),
			},
			{
				{Text: "Что такое уровни?", Data: shared.UniqueWhatLevels},
			},
		},
	}
}

func buildDetailsMessage(chatID, msgID int64) *tele.Message {
	return &tele.Message{
		ID:   int(msgID),
		Chat: &tele.Chat{ID: chatID},
		Text: "Реферальная программа👥\n\n[позже здесь будет информация о полученных бонусах и сроке их действия]\n\n А здесь — ссылка на сайт где прописаны все условия реферальной программы",
		Entities: tele.Entities{
			{Type: tele.EntityBold, Offset: 0, Length: 21},
			{Type: tele.EntityBold, Offset: 21, Length: 2},
			{Type: tele.EntityCustomEmoji, Offset: 21, Length: 2, CustomEmojiID: "5453957997418004470"},
			{Type: tele.EntityBold, Offset: 23, Length: 1},
		},
		ReplyMarkup: &tele.ReplyMarkup{
			InlineKeyboard: [][]tele.InlineButton{
				{{Text: "назад", Data: shared.UniqueBonusBack}},
			},
		},
	}
}

func buildWhatLevelsMessage(level models.LevelSummary, isAdmin bool, chatID int64) *tele.Message {
	botMentionLen := utils.TgLen("@ChatDetectiveBot")
	nextDecreaseText := "Ближайшего уменьшения нет"
	if level.NearestDecreaseAt > 0 {
		nextDecreaseText = fmt.Sprintf(
			"Ближайшее уменьшение: -%d уровня(ей) %s",
			level.NearestDecreaseAmount,
			time.Unix(level.NearestDecreaseAt, 0).Format("02.01.2006 15:04"),
		)
	}
	adminText := ""
	if isAdmin {
		adminText = "\nВы администратор: ваш приоритет выше любого обычного пользователя."
	}
	text := fmt.Sprintf(
		"УРОВНИ⬆️\n\n🦨Если ваш уровень выше уровня собеседника, только  вы будете получать уведомления о его действиях в переписке через @ChatDetectiveBot.\n\n🦨Если ваш уровень такой же, как у собеседника, то и вы, и собеседник будете получать уведомления о действиях противоположной стороны в переписке через @ChatDetectiveBot.\n\n🦨Если ваш уровень ниже уровня собеседника, то только ваш собеседник будет получать обновления о ваших действиях в переписке через @ChatDetectiveBot.\n\nВаш уровень сейчас: %d\n%s%s",
		level.Level,
		nextDecreaseText,
		adminText,
	)

	mention1Offset := utils.TgLen("УРОВНИ⬆️\n\n🦨Если ваш уровень выше уровня собеседника, только  вы будете получать уведомления о его действиях в переписке через ")
	mention2Offset := mention1Offset + botMentionLen + utils.TgLen(".\n\n🦨Если ваш уровень такой же, как у собеседника, то и вы, и собеседник будете получать уведомления о действиях противоположной стороны в переписке через ")
	mention3Offset := mention2Offset + botMentionLen + utils.TgLen(".\n\n🦨Если ваш уровень ниже уровня собеседника, то только ваш собеседник будет получать обновления о ваших действиях в переписке через ")

	return &tele.Message{
		Chat: &tele.Chat{ID: chatID},
		Text: text,
		Entities: tele.Entities{
			{Type: tele.EntityBold, Offset: 0, Length: 6},
			{Type: tele.EntityBold, Offset: 6, Length: 2},
			{Type: tele.EntityCustomEmoji, Offset: 6, Length: 2, CustomEmojiID: "5463122435425448565"},
			{Type: tele.EntityBold, Offset: 9, Length: 1},
			{Type: tele.EntityBold, Offset: 10, Length: 2},
			{Type: tele.EntityCustomEmoji, Offset: 10, Length: 2, CustomEmojiID: "5199660615978725258"},
			{Type: tele.EntityBold, Offset: 12, Length: 40},
			{Type: tele.EntityMention, Offset: mention1Offset, Length: botMentionLen},
			{Type: tele.EntityBold, Offset: 147, Length: 2},
			{Type: tele.EntityCustomEmoji, Offset: 147, Length: 2, CustomEmojiID: "5199660615978725258"},
			{Type: tele.EntityBold, Offset: 149, Length: 44},
			{Type: tele.EntityMention, Offset: mention2Offset, Length: botMentionLen},
			{Type: tele.EntityBold, Offset: 319, Length: 2},
			{Type: tele.EntityCustomEmoji, Offset: 319, Length: 2, CustomEmojiID: "5199660615978725258"},
			{Type: tele.EntityBold, Offset: 321, Length: 40},
			{Type: tele.EntityMention, Offset: mention3Offset, Length: botMentionLen},
		},
		ReplyMarkup: &tele.ReplyMarkup{
			InlineKeyboard: [][]tele.InlineButton{
				{{Text: "Повысить уровень", Data: shared.UniqueUpgradeLevel}},
			},
		},
	}
}

func buildLevelCommandMessage(level models.LevelSummary, chatID int64) *tele.Message {
	nextDecreaseText := "Ближайшего уменьшения нет"
	if level.NearestDecreaseAt > 0 {
		nextDecreaseText = fmt.Sprintf(
			"Дата ближайшего уменьшения: %s\nРазмер ближайшего уменьшения: -%d уровня(ей)",
			time.Unix(level.NearestDecreaseAt, 0).Format("02.01.2006 15:04"),
			level.NearestDecreaseAmount,
		)
	}

	return &tele.Message{
		Chat: &tele.Chat{ID: chatID},
		Text: fmt.Sprintf(
			"Текущий суммарный уровень: %d\n%s",
			level.Level,
			nextDecreaseText,
		),
		ReplyMarkup: &tele.ReplyMarkup{
			InlineKeyboard: [][]tele.InlineButton{
				{{Text: "Об уровнях", Data: shared.UniqueWhatLevels}},
			},
		},
	}
}

func runReferral(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	db := postgresql.GetDB()
	user, err := shared.GetUserByTgID(db, update.Message.Sender.ID)
	if e.IsNonNil(err) {
		return err
	}
	return hashe.Emit(shared.OutgoingRoutingKey, buildReferralMessage(user, update.Message.Chat.ID))
}

func runBonusSelect(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	db := postgresql.GetDB()
	user, settings, err := shared.GetUserByTgIDWithSettings(db, update.Callback.Sender.ID)
	if e.IsNonNil(err) {
		return err
	}
	_ = user

	msg := buildBonusSelectionMessage(
		update.Callback.Message.Chat.ID,
		int64(update.Callback.Message.ID),
		settings.ReferralBonusPreference,
	)
	if err := hashe.EmitEditMessage(shared.OutgoingRoutingKey, msg); e.IsNonNil(err) {
		return err
	}
	return hashe.EmitCallback(shared.OutgoingRoutingKey, update.Callback, shared.AnswerCallbackBanner("", update.Callback))
}

func runBonusDetails(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	msg := buildDetailsMessage(
		update.Callback.Message.Chat.ID,
		int64(update.Callback.Message.ID),
	)
	if err := hashe.EmitEditMessage(shared.OutgoingRoutingKey, msg); e.IsNonNil(err) {
		return err
	}
	return hashe.EmitCallback(shared.OutgoingRoutingKey, update.Callback, shared.AnswerCallbackBanner("", update.Callback))
}

func runBonusBack(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	db := postgresql.GetDB()
	user, err := shared.GetUserByTgID(db, update.Callback.Sender.ID)
	if e.IsNonNil(err) {
		return err
	}

	chatID := update.Callback.Message.Chat.ID
	msgID := int64(update.Callback.Message.ID)

	refMsg := buildReferralMessage(user, chatID)
	refMsg.ID = int(msgID)

	if err := hashe.EmitEditMessage(shared.OutgoingRoutingKey, refMsg); e.IsNonNil(err) {
		return err
	}
	return hashe.EmitCallback(shared.OutgoingRoutingKey, update.Callback, shared.AnswerCallbackBanner("", update.Callback))
}

func makeBonusTypeHandler(bonusType string) func(tele.Update, *h.HandlerChainHashe) *e.ErrorInfo {
	return func(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
		db := postgresql.GetDB()
		user, settings, err := shared.GetUserByTgIDWithSettings(db, update.Callback.Sender.ID)
		if e.IsNonNil(err) {
			return err
		}
		_ = user

		if settings.ReferralBonusPreference == bonusType {
			// Already selected — just acknowledge.
			return hashe.EmitCallback(shared.OutgoingRoutingKey, update.Callback, shared.AnswerCallbackBanner("", update.Callback))
		}

		settings.ReferralBonusPreference = bonusType
		_, eraw := db.Model(settings).WherePK().Column("referral_bonus_preference").Update()
		if eraw != nil {
			return e.FromError(eraw, "failed to update referral bonus preference").WithSeverity(e.Notice)
		}

		msg := buildBonusSelectionMessage(
			update.Callback.Message.Chat.ID,
			int64(update.Callback.Message.ID),
			bonusType,
		)
		if err := hashe.EmitEditMessage(shared.OutgoingRoutingKey, msg); e.IsNonNil(err) {
			return err
		}
		return hashe.EmitCallback(shared.OutgoingRoutingKey, update.Callback, shared.AnswerCallbackBanner("", update.Callback))
	}
}

func runWhatLevels(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	db := postgresql.GetDB()
	user, err := shared.GetUserByTgID(db, update.Callback.Sender.ID)
	if e.IsNonNil(err) {
		return err
	}

	level, err := models.GetUserLevelSummary(db, user.ID, time.Now())
	if e.IsNonNil(err) {
		return err
	}
	isAdmin, err := models.IsUserAdmin(db, user.ID)
	if e.IsNonNil(err) {
		return err
	}

	msg := buildWhatLevelsMessage(level, isAdmin, update.Callback.Message.Chat.ID)
	if err := hashe.Emit(shared.OutgoingRoutingKey, msg); e.IsNonNil(err) {
		return err
	}
	return hashe.EmitCallback(shared.OutgoingRoutingKey, update.Callback, shared.AnswerCallbackBanner("", update.Callback))
}

func runUpgradeLevel(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	if err := hashe.EmitCallback(
		shared.OutgoingRoutingKey,
		update.Callback,
		shared.AnswerCallbackBanner("Отправляю счёт", update.Callback),
	); e.IsNonNil(err) {
		return err
	}

	return emitLevelInvoice(update.Callback.Sender.ID, update.Callback.Message.Chat.ID, 1, hashe)
}

func runUpgradeLevelCommand(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	levels := 1
	if strings.HasPrefix(update.Message.Text, "/upgrade") {
		parsedLevels, err := parseUpgradeLevels(update.Message.Text)
		if e.IsNonNil(err) {
			return hashe.Emit(shared.OutgoingRoutingKey, buildUpgradeUsageMessage(update.Message.Chat.ID))
		}
		levels = parsedLevels
	}
	return emitLevelInvoice(update.Message.Sender.ID, update.Message.Chat.ID, levels, hashe)
}

func runLevelCommand(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	db := postgresql.GetDB()
	user, err := shared.GetUserByTgID(db, update.Message.Sender.ID)
	if e.IsNonNil(err) {
		return err
	}

	level, err := models.GetUserLevelSummary(db, user.ID, time.Now())
	if e.IsNonNil(err) {
		return err
	}

	return hashe.Emit(shared.OutgoingRoutingKey, buildLevelCommandMessage(level, update.Message.Chat.ID))
}

func parseUpgradeLevels(text string) (int, *e.ErrorInfo) {
	parts := strings.Fields(text)
	if len(parts) != 2 {
		return 0, e.NewError("invalid upgrade command args", "failed to parse upgrade command").WithSeverity(e.Notice)
	}
	levels, err := strconv.Atoi(parts[1])
	if err != nil || levels <= 0 {
		return 0, e.NewError("levels must be positive", "failed to parse upgrade command").WithSeverity(e.Notice)
	}
	return levels, e.Nil()
}

func emitLevelInvoice(tgUserID int64, chatID int64, levels int, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	paymentType := paymentservice.PaymentTypeLevelUp
	err, _ := paymentservice.EmitPayment(&paymentType, &paymentservice.PaymentOpts{
		MirrorID: hashe.MirrorID(),
		Recipient: &paymentservice.PaymentRecipientOpts{
			TelegramUserID: tgUserID,
			ChatID:         chatID,
		},
		Invoice: &paymentservice.PaymentInvoiceOpts{
			Title:       "Повышение уровня",
			Description: fmt.Sprintf("Покупка %d уровня(ей) на месяц", levels),
			PriceLabel:  fmt.Sprintf("%d уровня(ей)", levels),
		},
		LevelUp: &paymentservice.LevelUpOpts{Levels: levels},
	})
	if e.IsNonNil(err) {
		return err
	}
	return hashe.Emit(shared.OutgoingRoutingKey, buildUpgradeInvoiceSentMessage(chatID, levels))
}

func buildUpgradeUsageMessage(chatID int64) *tele.Message {
	return &tele.Message{
		Chat: &tele.Chat{ID: chatID},
		Text: "Используйте команду в формате: /upgrade 3",
	}
}

func buildUpgradeInvoiceSentMessage(chatID int64, levels int) *tele.Message {
	return &tele.Message{
		Chat: &tele.Chat{ID: chatID},
		Text: fmt.Sprintf("Счёт на +%d уровня(ей) отправлен. Уровни начислятся после оплаты.", levels),
	}
}
