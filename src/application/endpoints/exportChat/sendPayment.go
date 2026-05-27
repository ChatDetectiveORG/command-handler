package exportchat

import (
	"fmt"
	"time"

	shared "github.com/ChatDetectiveORG/command-handler/src/application/endpoints"
	"github.com/ChatDetectiveORG/command-handler/src/infrastructure/postgresql"
	paymentservice "github.com/ChatDetectiveORG/payment-service"
	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	models "github.com/ChatDetectiveORG/shared/postgresModels"
	"github.com/ChatDetectiveORG/shared/utils"

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

func runRestoreChat(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	if update.Callback == nil {
		return e.NewError("missing callback", "restore_chat requires callback").WithSeverity(e.Notice)
	}

	db := postgresql.GetDB()
	sender, err := shared.GetUserByTgID(db, update.Callback.Sender.ID)
	if e.IsNonNil(err) {
		return err.PushStack()
	}

	data := utils.ParseCallbackData(update.Callback.Data)
	code, ok := data[shared.CallbackFieldCode]
	if !ok {
		return hashe.EmitCallback(shared.OutgoingRoutingKey, update.Callback, shared.AnswerCallbackBanner("Кнопка устарела, попробуйте снова из списка чатов.", update.Callback))
	}

	interlocutor := &models.Telegramuser{}
	if eRaw := db.Model(interlocutor).Where("referral_code = ?", code).Select(); e.IsNonNil(eRaw) {
		return e.FromError(eRaw, "failed to load interlocutor by referral code").WithSeverity(e.Notice)
	}

	err = checkCallbackPermission(sender, interlocutor, db)
	if e.IsNonNil(err) {
		err = hashe.EmitCallback(shared.OutgoingRoutingKey, update.Callback, shared.AnswerCallbackBanner("У вас нет доступа к этой странице", update.Callback))

		return err
	}

	count, err := chatMessageCount(db, sender, interlocutor)
	if e.IsNonNil(err) {
		return err.PushStack()
	}
	if count <= 0 {
		return hashe.EmitCallback(shared.OutgoingRoutingKey, update.Callback, shared.AnswerCallbackBanner("В этом чате нет сообщений для восстановления.", update.Callback))
	}

	paymentType := paymentservice.PaymentTypeExportChat
	emitErr, _ := paymentservice.EmitPayment(&paymentType, &paymentservice.PaymentOpts{
		MirrorID: hashe.MirrorID(),
		Recipient: &paymentservice.PaymentRecipientOpts{
			TelegramUserID: update.Callback.Sender.ID,
			ChatID:         update.Callback.Message.Chat.ID,
		},
		Invoice: &paymentservice.PaymentInvoiceOpts{
			Title:       "Восстановление чата",
			Description: fmt.Sprintf("Восстановление переписки (%d сообщений)", count),
			PriceLabel:  fmt.Sprintf("%d сообщений", count),
		},
		ExportChat: &paymentservice.ExportChatOpts{
			Messages:         count,
			InterlocutorCode: code,
			SenderIDHash:     sender.IDHash,
			StatusChatID:     update.Callback.Message.Chat.ID,
		},
	})
	if e.IsNonNil(emitErr) {
		// Release the lock so the user can retry; payment never went out.
		// _ = cdredis.ReleaseExportLock(sender.IDHash)
		return emitErr.PushStack()
	}

	if err := hashe.EmitCallback(shared.OutgoingRoutingKey, update.Callback, shared.AnswerCallbackBanner("Счёт отправлен, оплатите его для запуска восстановления.", update.Callback)); e.IsNonNil(err) {
		return err.PushStack()
	}
	return e.Nil()
}
