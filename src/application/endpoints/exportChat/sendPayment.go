package exportchat

import (
	"fmt"
	"strings"
	"time"

	shared "github.com/ChatDetectiveORG/command-handler/src/application/endpoints"
	"github.com/ChatDetectiveORG/command-handler/src/infrastructure/postgresql"
	cdredis "github.com/ChatDetectiveORG/command-handler/src/infrastructure/redis"
	paymentservice "github.com/ChatDetectiveORG/payment-service"
	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	models "github.com/ChatDetectiveORG/shared/postgresModels"

	tele "gopkg.in/telebot.v4"
)

func NewRestoreChatEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"restore_chat",
		*h.HandlerChain{}.Init(
			30*time.Second,
			h.InitChainHandler(runRestoreChat, h.EndOnError),
		),
		h.CallbackStartsWith(shared.UniqueRestoreChat),
	)
	return ep
}

// extractMetaID strips the unique prefix and the "\n" separator that UniqueCallback expects.
func extractMetaID(callbackData string) string {
	rest := strings.TrimPrefix(callbackData, shared.UniqueRestoreChat)
	rest = strings.TrimPrefix(rest, "\n")
	return rest
}

func runRestoreChat(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	cb := update.Callback
	if cb == nil {
		return e.NewError("missing callback", "restore_chat requires callback").WithSeverity(e.Notice)
	}

	metaID := extractMetaID(cb.Data)
	if metaID == "" {
		return hashe.EmitCallback(shared.OutgoingRoutingKey, cb, shared.AnswerCallbackBanner("Кнопка устарела, попробуйте снова из списка чатов.", cb))
	}

	var meta restoreCallbackMeta
	if err := cdredis.LoadCallbackMeta(metaID, &meta); e.IsNonNil(err) {
		return hashe.EmitCallback(shared.OutgoingRoutingKey, cb, shared.AnswerCallbackBanner("Кнопка устарела, попробуйте снова из списка чатов.", cb))
	}

	db := postgresql.GetDB()
	sender, err := shared.GetUserByTgID(db, cb.Sender.ID)
	if e.IsNonNil(err) {
		return err.PushStack()
	}

	interlocutor := &models.Telegramuser{}
	if eRaw := db.Model(interlocutor).Where("referral_code = ?", meta.InterlocutorCode).Select(); e.IsNonNil(eRaw) {
		return e.FromError(eRaw, "failed to load interlocutor by referral code").WithSeverity(e.Notice)
	}

	count, err := chatMessageCount(db, sender, interlocutor)
	if e.IsNonNil(err) {
		return err.PushStack()
	}
	if count <= 0 {
		return hashe.EmitCallback(shared.OutgoingRoutingKey, cb, shared.AnswerCallbackBanner("В этом чате нет сообщений для восстановления.", cb))
	}

	// acquired, err := cdredis.AcquireExportLock(sender.IDHash)
	// if e.IsNonNil(err) {
	// 	return err.PushStack()
	// }
	// if !acquired {
	// 	return hashe.EmitCallback(shared.OutgoingRoutingKey, cb, shared.AnswerCallbackBanner("У тебя уже идёт экспорт, дождись его завершения.", cb))
	// }

	paymentType := paymentservice.PaymentTypeExportChat
	emitErr, _ := paymentservice.EmitPayment(&paymentType, &paymentservice.PaymentOpts{
		MirrorID: hashe.MirrorID(),
		Recipient: &paymentservice.PaymentRecipientOpts{
			TelegramUserID: cb.Sender.ID,
			ChatID:         cb.Message.Chat.ID,
		},
		Invoice: &paymentservice.PaymentInvoiceOpts{
			Title:       "Восстановление чата",
			Description: fmt.Sprintf("Восстановление переписки (%d сообщений)", count),
			PriceLabel:  fmt.Sprintf("%d сообщений", count),
		},
		ExportChat: &paymentservice.ExportChatOpts{
			Messages:         count,
			InterlocutorCode: meta.InterlocutorCode,
			SenderIDHash:     sender.IDHash,
			StatusChatID:     cb.Message.Chat.ID,
		},
	})
	if e.IsNonNil(emitErr) {
		// Release the lock so the user can retry; payment never went out.
		// _ = cdredis.ReleaseExportLock(sender.IDHash)
		return emitErr.PushStack()
	}

	if err := hashe.EmitCallback(shared.OutgoingRoutingKey, cb, shared.AnswerCallbackBanner("Счёт отправлен, оплатите его для запуска восстановления.", cb)); e.IsNonNil(err) {
		return err.PushStack()
	}
	return e.Nil()
}
