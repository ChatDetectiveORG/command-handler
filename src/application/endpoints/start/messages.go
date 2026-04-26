package start

import (
	"fmt"
	"strings"

	shared "github.com/ChatDetectiveORG/command-handler/src/application/endpoints"
	"github.com/ChatDetectiveORG/command-handler/src/infrastructure/postgresql"
	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	models "github.com/ChatDetectiveORG/shared/postgresModels"
	utils "github.com/ChatDetectiveORG/shared/utils"
	"github.com/go-pg/pg/v10"
	tele "gopkg.in/telebot.v4"
)

const showContactsUnique = shared.UniqueShowContacts

// exampleUsernameLen is the UTF-16 length of the example username used in the spec:
// "⊹🍌𝙻𝚒𝚂𝚂𝚒𝙺𝚔🔮⊹" = 20 UTF-16 code units.
const exampleUsernameLen = 20

// baseStartEntities are the message entities from the spec, with offsets matching
// the example text where the username is 20 UTF-16 units long.
var baseStartEntities = []tele.MessageEntity{
	{Type: tele.EntityCustomEmoji, Offset: 31, Length: 2, CustomEmojiID: "5368302880518323242"},
	{Type: tele.EntityCustomEmoji, Offset: 113, Length: 2, CustomEmojiID: "5465465194056525619"},
	{Type: tele.EntityBold, Offset: 115, Length: 51},
	{Type: tele.EntityBold, Offset: 166, Length: 2},
	{Type: tele.EntityCustomEmoji, Offset: 166, Length: 2, CustomEmojiID: "5267402850215936014"},
	{Type: tele.EntityCustomEmoji, Offset: 169, Length: 2, CustomEmojiID: "5465465194056525619"},
	{Type: tele.EntityBold, Offset: 171, Length: 40},
	{Type: tele.EntityBold, Offset: 211, Length: 1},
	{Type: tele.EntityCustomEmoji, Offset: 211, Length: 1, CustomEmojiID: "5373221902267457807"},
	{Type: tele.EntityCustomEmoji, Offset: 213, Length: 2, CustomEmojiID: "5465465194056525619"},
	{Type: tele.EntityBold, Offset: 215, Length: 34},
	{Type: tele.EntityBold, Offset: 249, Length: 1},
	{Type: tele.EntityCustomEmoji, Offset: 249, Length: 1, CustomEmojiID: "5366252188253303469"},
	{Type: tele.EntityUnderline, Offset: 252, Length: 23},
	{Type: tele.EntityBold, Offset: 252, Length: 22},
	{Type: tele.EntityBold, Offset: 275, Length: 39},
	{Type: tele.EntityUnderline, Offset: 275, Length: 39},
	{Type: tele.EntityCustomEmoji, Offset: 314, Length: 2, CustomEmojiID: "5429405838345265327"},
}

// startMessageSuffix is the part of the /start text that follows the user's first name.
const startMessageSuffix = " ! 👋 \n\nЭтот бот создан, чтобы отслеживать действия твоих собеседников в переписке.\n\n👍Узнай, если собеседник изменит или удалит сообщение👁\n👍Скачивает фото, видео и аудио с таймером⏳\n👍Работает даже без Telegram Premium⭐\n\nБот полностью анонимен и надёжно шифрует всю информацию о тебе🔓\nТакже в любой момент ты можешь запросить моментальное удаление всей информации о себе"

func buildWelcomeMessage(tgUser *tele.User, chatID int64) *tele.Message {
	firstName := tgUser.FirstName
	nameLen := utils.TgLen(firstName)
	delta := nameLen - exampleUsernameLen

	return &tele.Message{
		Chat:     &tele.Chat{ID: chatID},
		Text:     "Привет, " + firstName + startMessageSuffix,
		Entities: shared.ShiftEntities(baseStartEntities, delta),
		ReplyMarkup: &tele.ReplyMarkup{
			ResizeKeyboard: true,
			ReplyKeyboard: [][]tele.ReplyButton{
				{{Text: shared.BtnInstallGuide}, {Text: shared.BtnCheckConnection}},
				{{Text: shared.BtnSettings}},
				{{Text: shared.BtnInviteFriend}},
				{{Text: shared.BtnUpgradeLevel}, {Text: shared.BtnHowEncryption}},
			},
		},
	}
}

func buildNonPremiumNoticeMessage(chatID int64) *tele.Message {
	return &tele.Message{
		Chat: &tele.Chat{ID: chatID},
		Text: "☝️Без подписки Telegram Premium бот может работать только в тех чатах, где собеседник использует бота. Нажми на кнопку ниже, чтобы узнать, в каких чатах бот будет работать",
		Entities: tele.Entities{
			{Type: tele.EntityCustomEmoji, Offset: 0, Length: 2, CustomEmojiID: "5453958478454341679"},
		},
		ReplyMarkup: &tele.ReplyMarkup{
			InlineKeyboard: [][]tele.InlineButton{
				{{Text: "Показать список", Data: showContactsUnique}},
			},
		},
	}
}

// buildContactsMessage constructs the "list of bot contacts" message for non-premium users.
func buildContactsMessage(user *models.Telegramuser, contacts []*models.Telegramuser, chatID int64) (*tele.Message, *e.ErrorInfo) {
	if len(contacts) == 0 {
		return buildNoContactsMessage(user, chatID)
	}

	refLink := shared.BuildReferralLink(user)

	var sb strings.Builder
	var entities tele.Entities

	prefix := "🔓Бот будет работать в переписках с\n"
	sb.WriteString(prefix)
	entities = append(entities, tele.MessageEntity{
		Type:          tele.EntityCustomEmoji,
		Offset:        0,
		Length:        2,
		CustomEmojiID: "5465443379917629504",
	})

	for _, contact := range contacts {
		offset := utils.TgLen(sb.String())
		tgID, err := contact.GetTgId()
		if e.IsNonNil(err) {
			continue
		}
		fullName, err := contact.GetFullName()
		if e.IsNonNil(err) {
			continue
		}
		fullName = strings.TrimSpace(fullName)
		nameLen := utils.TgLen(fullName)
		sb.WriteString(fullName)
		sb.WriteString(",\n")
		entities = append(entities, tele.MessageEntity{
			Type:   tele.EntityTMention,
			Offset: offset,
			Length: nameLen,
			User:   &tele.User{ID: tgID},
		})
	}

	// "\n👥Не нашёл..." suffix
	groupPart := "\n👥Не нашёл здесь того, кого хотел? Пригласи его по реферальной программе и получи "
	groupEmojiOffset := utils.TgLen(sb.String()) + utils.TgLen("\n")
	sb.WriteString(groupPart)
	sb.WriteString(fmt.Sprintf("%d рублей\n", shared.ReferralBonusRub))
	sb.WriteString("Он получит преимущества использования бота, а ты сможешь просматривать удалённые сообщения в чате с ним")

	handshakeOffset := utils.TgLen(sb.String())
	sb.WriteString("🤝\n")
	arrowOffset := utils.TgLen(sb.String())
	sb.WriteString("👉Ссылка: ")
	linkOffset := utils.TgLen(sb.String())
	sb.WriteString(refLink)
	refLinkLen := utils.TgLen(refLink)

	entities = append(entities,
		tele.MessageEntity{Type: tele.EntityCustomEmoji, Offset: groupEmojiOffset, Length: 2, CustomEmojiID: "5453957997418004470"},
		tele.MessageEntity{Type: tele.EntityCustomEmoji, Offset: handshakeOffset, Length: 2, CustomEmojiID: "5463256910851546817"},
		tele.MessageEntity{Type: tele.EntityCustomEmoji, Offset: arrowOffset, Length: 2, CustomEmojiID: "5368574485660187071"},
		tele.MessageEntity{Type: tele.EntityURL, Offset: linkOffset, Length: refLinkLen},
	)

	return &tele.Message{
		Chat:           &tele.Chat{ID: chatID},
		Text:           sb.String(),
		Entities:       entities,
		PreviewOptions: &tele.PreviewOptions{Disabled: true},
	}, e.Nil()
}

func buildNoContactsMessage(user *models.Telegramuser, chatID int64) (*tele.Message, *e.ErrorInfo) {
	refLink := shared.BuildReferralLink(user)

	var sb strings.Builder
	var entities tele.Entities

	// "🔋Никто из..."
	sb.WriteString("🔋Никто из твоих собеседников не использует ")
	entities = append(entities, tele.MessageEntity{
		Type:          tele.EntityCustomEmoji,
		Offset:        0,
		Length:        2,
		CustomEmojiID: "5454125707300978880",
	})

	mentionOffset := utils.TgLen(sb.String())
	mentionLen := utils.TgLen(shared.BotUsername)
	sb.WriteString(shared.BotUsername)
	entities = append(entities, tele.MessageEntity{
		Type:   tele.EntityMention,
		Offset: mentionOffset,
		Length: mentionLen,
	})

	sb.WriteString(". К сожалению, для работы бота без Telegram Premium необходимо, чтобы хотя бы у одного человека в чате был подключён бот\n\n")

	groupEmojiOffset := utils.TgLen(sb.String())
	sb.WriteString("👥Хочешь это исправить? Пригласи друзей по реферальной программе и получи ")
	sb.WriteString(fmt.Sprintf("%d рублей\n", shared.ReferralBonusRub))
	sb.WriteString("Они получат преимущества использования бота, а ты сможешь просматривать удалённые сообщения в чате с ними")

	handshakeOffset := utils.TgLen(sb.String())
	sb.WriteString("🤝\n")
	arrowOffset := utils.TgLen(sb.String())
	sb.WriteString("👉Ссылка: ")
	linkOffset := utils.TgLen(sb.String())
	sb.WriteString(refLink)
	refLinkLen := utils.TgLen(refLink)

	entities = append(entities,
		tele.MessageEntity{Type: tele.EntityCustomEmoji, Offset: groupEmojiOffset, Length: 2, CustomEmojiID: "5453957997418004470"},
		tele.MessageEntity{Type: tele.EntityCustomEmoji, Offset: handshakeOffset, Length: 2, CustomEmojiID: "5463256910851546817"},
		tele.MessageEntity{Type: tele.EntityCustomEmoji, Offset: arrowOffset, Length: 2, CustomEmojiID: "5368574485660187071"},
		tele.MessageEntity{Type: tele.EntityURL, Offset: linkOffset, Length: refLinkLen},
	)

	return &tele.Message{
		Chat:           &tele.Chat{ID: chatID},
		Text:           sb.String(),
		Entities:       entities,
		PreviewOptions: &tele.PreviewOptions{Disabled: true},
	}, e.Nil()
}

func run(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	db := postgresql.GetDB()
	tgUser := update.Message.Sender
	chatID := update.Message.Chat.ID

	tx, eraw := db.Begin()
	if eraw != nil {
		return e.FromError(eraw, "failed to begin transaction").WithSeverity(e.Critical)
	}
	defer tx.Rollback()

	user := &models.Telegramuser{}
	if err := user.GetOrCreate(tx, tgUser); e.IsNonNil(err) {
		return err
	}

	// Always refresh name/username/metadata. For new users this is a no-op in practice;
	// for returning users it keeps profile data up to date.
	_ = user.UpdateUserData(tx, tgUser)

	if eraw = tx.Commit(); eraw != nil {
		return e.FromError(eraw, "failed to commit transaction").WithSeverity(e.Critical)
	}

	if err := hashe.Emit(shared.OutgoingRoutingKey, buildWelcomeMessage(tgUser, chatID)); e.IsNonNil(err) {
		return err
	}

	if !tgUser.IsPremium {
		if err := hashe.Emit(shared.OutgoingRoutingKey, buildNonPremiumNoticeMessage(chatID)); e.IsNonNil(err) {
			return err
		}
	}

	return e.Nil()
}

func runShowContacts(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	db := postgresql.GetDB()
	chatID := update.Callback.Sender.ID

	user, err := shared.GetUserByTgID(db, chatID)
	if e.IsNonNil(err) {
		return err
	}

	relations, err := shared.ContactsForUser(db, user)
	if e.IsNonNil(err) {
		return err
	}

	var contacts []*models.Telegramuser
	for i := range relations {
		other := shared.OtherUserInRelation(relations[i], user)
		if other != nil {
			contacts = append(contacts, other)
		}
	}

	msg, err := buildContactsMessage(user, contacts, chatID)
	if e.IsNonNil(err) {
		return err
	}

	if emitErr := hashe.Emit(shared.OutgoingRoutingKey, msg); e.IsNonNil(emitErr) {
		return emitErr
	}

	return hashe.EmitCallback(
		shared.OutgoingRoutingKey,
		update.Callback,
		shared.AnswerCallbackBanner("", update.Callback),
	)
}

// GetContactsMessageForUser is exported for reuse in businessConnection and checkConnection handlers.
func GetContactsMessageForUser(db *pg.DB, user *models.Telegramuser, chatID int64) (*tele.Message, *e.ErrorInfo) {
	relations, err := shared.ContactsForUser(db, user)
	if e.IsNonNil(err) {
		return nil, err
	}

	var contacts []*models.Telegramuser
	for i := range relations {
		other := shared.OtherUserInRelation(relations[i], user)
		if other != nil {
			contacts = append(contacts, other)
		}
	}

	return buildContactsMessage(user, contacts, chatID)
}
