package mirror

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	shared "github.com/ChatDetectiveORG/command-handler/src/application/endpoints"
	"github.com/ChatDetectiveORG/command-handler/src/infrastructure/config"
	"github.com/ChatDetectiveORG/command-handler/src/infrastructure/postgresql"
	redisinfra "github.com/ChatDetectiveORG/command-handler/src/infrastructure/redis"
	paymentservice "github.com/ChatDetectiveORG/payment-service"
	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	models "github.com/ChatDetectiveORG/shared/postgresModels"
	"github.com/gomodule/redigo/redis"
	"github.com/google/uuid"
	tele "gopkg.in/telebot.v4"
)

const (
	waitStateTTLSeconds = 10 * 60
)

var tokenFormat = regexp.MustCompile(`^[0-9]{6,}:[A-Za-z0-9_-]{20,}$`)

func NewCreateMirrorEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"create_mirror",
		*h.HandlerChain{}.Init(
			3*time.Minute,
			h.InitChainHandler(runCreateMirror, h.EndOnError),
		),
		h.Command([]string{"create_mirror"}),
	)
	return ep
}

func NewMirrorTokenEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"create_mirror_token",
		*h.HandlerChain{}.Init(
			3*time.Minute,
			h.InitChainHandler(runMirrorToken, h.EndOnError),
		),
		h.Text(),
	)
	return ep
}

func runCreateMirror(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	if update.Message == nil || update.Message.Chat == nil || update.Message.Sender == nil {
		return e.NewError("message is empty", "failed to create mirror").WithSeverity(e.Notice)
	}
	if err := setWaitingForToken(update.Message.Chat.ID); e.IsNonNil(err) {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_, err := hashe.EmitMessageUserAnswer(ctx, shared.OutgoingRoutingKey, buildInstructionMessage(update.Message))
	return err
}

func runMirrorToken(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	if update.Message == nil || update.Message.Chat == nil || update.Message.Sender == nil {
		return e.Nil()
	}
	text := strings.TrimSpace(update.Message.Text)
	if strings.HasPrefix(text, "/create_mirror") {
		return e.Nil()
	}

	waiting, err := isWaitingForToken(update.Message.Chat.ID)
	if e.IsNonNil(err) {
		return err
	}
	if !waiting {
		return e.Nil()
	}
	if text == "/cancel" || strings.EqualFold(text, "cancel") {
		if err := clearWaitingForToken(update.Message.Chat.ID); e.IsNonNil(err) {
			return err
		}
		return hashe.Emit(shared.OutgoingRoutingKey, textMessage(update.Message.Chat.ID, "Создание зеркала отменено."))
	}
	if !tokenFormat.MatchString(text) {
		return hashe.Emit(shared.OutgoingRoutingKey, textMessage(update.Message.Chat.ID, "Токен должен выглядеть так: 1234567890:ABCdEFgHIjKLmNoPQRstUVwXyZ123456789\n\nНе удалось проверить токен. Отправьте его ещё раз или введите /cancel."))
	}

	if err := clearWaitingForToken(update.Message.Chat.ID); e.IsNonNil(err) {
		return err
	}
	return createMirrorFromToken(update, hashe, text)
}

func createMirrorFromToken(update tele.Update, hashe *h.HandlerChainHashe, token string) *e.ErrorInfo {
	bot, rawErr := tele.NewBot(tele.Settings{Token: token, Poller: nil})
	if rawErr != nil {
		return hashe.Emit(shared.OutgoingRoutingKey, textMessage(update.Message.Chat.ID, "Не удалось проверить токен. Проверьте его и отправьте команду /create_mirror ещё раз."))
	}
	if bot.Me == nil || !bot.Me.CanConnectToBusiness {
		return hashe.Emit(shared.OutgoingRoutingKey, textMessage(update.Message.Chat.ID, "У этого бота не включён Бизнес-режим. Включите его в @BotFather и попробуйте снова."))
	}

	db := postgresql.GetDB()
	owner, err := shared.GetUserByTgID(db, update.Message.Sender.ID)
	if e.IsNonNil(err) {
		return err
	}
	now := time.Now()
	activeCount, err := models.CountActiveMirrorsForOwner(db, owner.ID, now)
	if e.IsNonNil(err) {
		return err
	}

	unique := uuid.New().String()
	status := models.MirrorStatusActive
	if activeCount >= mirrorFreeLimit() {
		status = models.MirrorStatusPending
	}
	mirror, err := models.NewMirror(owner, token, bot.Me.ID, bot.Me.Username, unique, status, now)
	if e.IsNonNil(err) {
		return err
	}
	if _, rawErr := db.Model(mirror).Insert(); rawErr != nil {
		return e.FromError(rawErr, "failed to create mirror").WithSeverity(e.Notice)
	}

	if status == models.MirrorStatusPending {
		return emitMirrorPayment(update, hashe, mirror.ID)
	}
	if err := setMirrorWebhook(bot, unique); e.IsNonNil(err) {
		return err
	}
	return hashe.Emit(shared.OutgoingRoutingKey, buildSuccessMessage(update.Message.Chat.ID, bot.Me.Username))
}

func emitMirrorPayment(update tele.Update, hashe *h.HandlerChainHashe, mirrorID int) *e.ErrorInfo {
	paymentType := paymentservice.PaymentTypeMirror
	err, _ := paymentservice.EmitPayment(&paymentType, &paymentservice.PaymentOpts{
		MirrorID: hashe.MirrorID(),
		Recipient: &paymentservice.PaymentRecipientOpts{
			TelegramUserID: update.Message.Sender.ID,
			ChatID:         update.Message.Chat.ID,
		},
		Invoice: &paymentservice.PaymentInvoiceOpts{
			Title:       "Дополнительное зеркало",
			Description: "Ежемесячная доплата за зеркало сверх лимита",
			PriceLabel:  "1 зеркало",
		},
		Mirror: &paymentservice.MirrorOpts{PendingMirrorID: mirrorID},
	})
	if e.IsNonNil(err) {
		return err
	}
	return hashe.Emit(shared.OutgoingRoutingKey, textMessage(update.Message.Chat.ID, "Бесплатный лимит зеркал исчерпан. Отправил счёт на ежемесячную доплату."))
}

func setMirrorWebhook(bot *tele.Bot, unique string) *e.ErrorInfo {
	if bot == nil {
		return e.NewError("bot is nil", "failed to set mirror webhook").WithSeverity(e.Notice)
	}
	webhook := &tele.Webhook{
		Endpoint:       &tele.WebhookEndpoint{PublicURL: buildMirrorWebhookURL(unique)},
		MaxConnections: 100,
		AllowedUpdates: []string{
			"message",
			"callback_query",
			"shipping_query",
			"pre_checkout_query",
			"business_connection",
			"business_message",
			"edited_business_message",
			"deleted_business_messages",
		},
	}
	if err := bot.SetWebhook(webhook); err != nil {
		return e.FromError(err, "failed to set mirror webhook").WithSeverity(e.Notice)
	}
	return e.Nil()
}

func buildMirrorWebhookURL(unique string) string {
	base := strings.TrimRight(os.Getenv("TELEGRAM_BOT_PUBLIC_URL"), "/")
	return base + "/mirror/" + unique
}

func mirrorFreeLimit() int {
	value := strings.TrimSpace(os.Getenv("MIRROR_FREE_LIMIT"))
	if value == "" {
		return 1
	}
	limit, err := strconv.Atoi(value)
	if err != nil || limit < 0 {
		return 1
	}
	return limit
}

func buildInstructionMessage(replyTo *tele.Message) *tele.Message {
	text := "Инструкция по созданию бота🤖\n\n1. Открой в Telegram официального отца всех ботов — @BotFather. Нажми Start, а затем отправь команду /newbot. Придумай и отправь имя для бота (как он будет называться в списке контактов), затем придумай и отправь его username (адрес, по которому его можно найти, например MyBusinessBot). Он обязательно должен заканчиваться на _bot.\n\n2. Включи Бизнес-режим. Чтобы бот мог работать, нужно дать ему несколько разрешений: В том же @BotFather отправь команду /mybots, выбери своего нового бота из списка, нажми Bot Settings -> Business Mode. Нажми Turn on, чтобы статус изменился на «Enabled».\n\n3. Передай ключ. После создания бота @BotFather прислал тебе сообщение с текстом HTTP API token (длинная строка из цифр и букв формата 1234567890:ABCdEFgHIjKLmNoPQRstUVwXyZ123456789). Пожалуйста, скопируй этот токен и отправь его в чат ниже."
	return &tele.Message{
		Chat:    replyTo.Chat,
		ReplyTo: replyTo,
		Text:    text,
		Entities: tele.Entities{
			{Offset: 0, Length: 27, Type: tele.EntityBold},
			{Offset: 27, Length: 2, Type: tele.EntityBold},
			{Offset: 27, Length: 2, Type: tele.EntityCustomEmoji, CustomEmojiID: "5296447931627352804"},
			{Offset: 34, Length: 49, Type: tele.EntityBold},
			{Offset: 83, Length: 10, Type: tele.EntityMention},
			{Offset: 83, Length: 10, Type: tele.EntityBold},
			{Offset: 93, Length: 1, Type: tele.EntityBold},
			{Offset: 132, Length: 7, Type: tele.EntityCommand},
			{Offset: 368, Length: 20, Type: tele.EntityBold},
			{Offset: 459, Length: 10, Type: tele.EntityMention},
			{Offset: 486, Length: 7, Type: tele.EntityCommand},
			{Offset: 622, Length: 15, Type: tele.EntityBold},
			{Offset: 659, Length: 10, Type: tele.EntityMention},
			{Offset: 757, Length: 10, Type: tele.EntityPhone},
		},
	}
}

func buildSuccessMessage(chatID int64, botUsername string) *tele.Message {
	botMention := "@" + botUsername
	text := fmt.Sprintf("Зеркало успешно создано!👉\n\nОно функционирует точно так же, как и %s.\nЕсли он не будет использоваться больше месяца, то %s будет отключён от системы", shared.BotUsername, botMention)
	return &tele.Message{
		Chat: &tele.Chat{ID: chatID},
		Text: text,
		Entities: tele.Entities{
			{Offset: 24, Length: 2, Type: tele.EntityCustomEmoji, CustomEmojiID: "5463392464314315076"},
			{Offset: 66, Length: 25, Type: tele.EntityMention},
		},
	}
}

func textMessage(chatID int64, text string) *tele.Message {
	return &tele.Message{Chat: &tele.Chat{ID: chatID}, Text: text}
}

func setWaitingForToken(chatID int64) *e.ErrorInfo {
	conn, err := redisConn()
	if e.IsNonNil(err) {
		return err
	}
	defer conn.Close()
	_, rawErr := conn.Do("SETEX", waitKey(chatID), waitStateTTLSeconds, "1")
	if rawErr != nil {
		return e.FromError(rawErr, "failed to set mirror wait state").WithSeverity(e.Notice)
	}
	return e.Nil()
}

func isWaitingForToken(chatID int64) (bool, *e.ErrorInfo) {
	conn, err := redisConn()
	if e.IsNonNil(err) {
		return false, err
	}
	defer conn.Close()
	exists, rawErr := redis.Bool(conn.Do("EXISTS", waitKey(chatID)))
	if rawErr != nil {
		return false, e.FromError(rawErr, "failed to get mirror wait state").WithSeverity(e.Notice)
	}
	return exists, e.Nil()
}

func clearWaitingForToken(chatID int64) *e.ErrorInfo {
	conn, err := redisConn()
	if e.IsNonNil(err) {
		return err
	}
	defer conn.Close()
	_, rawErr := conn.Do("DEL", waitKey(chatID))
	if rawErr != nil {
		return e.FromError(rawErr, "failed to clear mirror wait state").WithSeverity(e.Notice)
	}
	return e.Nil()
}

func redisConn() (redis.Conn, *e.ErrorInfo) {
	cfg, err := config.FetchConfig()
	if e.IsNonNil(err) {
		return nil, err
	}
	pool, err := redisinfra.GetPool(cfg)
	if e.IsNonNil(err) {
		return nil, err
	}
	return pool.Get(), e.Nil()
}

func waitKey(chatID int64) string {
	return fmt.Sprintf("mirror:create:%d", chatID)
}
