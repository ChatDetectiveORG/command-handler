package mirror

import (
	"encoding/json"
	"fmt"
	"time"

	shared "github.com/ChatDetectiveORG/command-handler/src/application/endpoints"
	"github.com/ChatDetectiveORG/command-handler/src/infrastructure/postgresql"
	paymentservice "github.com/ChatDetectiveORG/payment-service"
	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	models "github.com/ChatDetectiveORG/shared/postgresModels"
	tele "gopkg.in/telebot.v4"
)

func NewMirrorListEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"mirror_list",
		*h.HandlerChain{}.Init(
			30*time.Second,
			h.InitChainHandler(runMirrorList, h.EndOnError),
		),
		h.Or(
			h.UniqueCallback(shared.UniqueMirrorList),
			h.Command([]string{"mirrors"}),
		),
	)
	return ep
}

func buildMirrorsKeyboard(senderID int64) (*tele.ReplyMarkup, *e.ErrorInfo) {
	db := postgresql.GetDB()
	user, err := shared.GetUserByTgID(db, senderID)
	if e.IsNonNil(err) {
		return &tele.ReplyMarkup{}, err
	}

	mirrors, err := models.GetAllMirrorsForOwner(db, user.ID)
	if e.IsNonNil(err) {
		return &tele.ReplyMarkup{}, err
	}

	buildMirrorButton := func(mirror *models.Mirror) []tele.InlineButton {
		statusIconID := shared.EmojiMirrorActive
		if mirror.Status != models.MirrorStatusActive {
			statusIconID = shared.EmojiMirrorInactive
		}

		var costsText string = "Free"

		var lastTransaction = &models.Payment{}
		err := e.Wrap(db.Model(lastTransaction).Where("id = ?", mirror.SourcePaymentID).Select())
		if e.IsNil(err) {
			costsText = fmt.Sprintf("%d %s", lastTransaction.Amount, paymentservice.GetCurrencyName(lastTransaction.Currency))
		}

		if mirror.Status == models.MirrorStatusPending {
			costsText = "Ожидает оплаты"
		}

		data, eRaw := json.Marshal(map[string]any{
			"mirror_id": mirror.ID,
		})

		if e.IsNonNil(eRaw) {
			return []tele.InlineButton{}
		}

		return []tele.InlineButton{{
			Text:              fmt.Sprintf("%s [%s]", mirror.BotUsername, costsText),
			Data:              shared.UniqueMirrorDetails + string(data),
			IconCustomEmojiID: statusIconID,
		}}
	}

	buttons := make([][]tele.InlineButton, 0, len(mirrors))
	for _, mirror := range mirrors {
		buttons = append(buttons, buildMirrorButton(mirror))
	}

	return &tele.ReplyMarkup{
		InlineKeyboard: buttons,
	}, e.Nil()
}

func buildMirrorsMessage(chatID int64, senderID int64, msgID int) *tele.Message {
	replyMarkup, err := buildMirrorsKeyboard(senderID)
	if e.IsNonNil(err) {
		return nil
	}

	if len(replyMarkup.InlineKeyboard) == 0 {
		return &tele.Message{
			ID:   msgID,
			Chat: &tele.Chat{ID: chatID},
			Text: "![🪩](tg://emoji?id=5257960214291823402)*Список зеркал*\n\n_У вас нет зеркал_",
		}
	}

	return &tele.Message{
		ID:          msgID,
		Chat:        &tele.Chat{ID: chatID},
		Text:        "![🪩](tg://emoji?id=5257960214291823402)*Список зеркал*",
		ReplyMarkup: replyMarkup,
	}
}

func runMirrorList(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	if update.Callback != nil {
		update.Message = update.Callback.Message
		update.Message.Sender.ID = update.Callback.Sender.ID
	}

	message := buildMirrorsMessage(update.Message.Chat.ID, update.Message.Sender.ID, update.Message.ID)
	if message == nil {
		return e.NewError("failed to build mirrors message", "failed to run mirror list endpoint").WithSeverity(e.Notice)
	}

	if update.Callback != nil {
		return hashe.WithParseMode(true).EmitEditMessage(shared.OutgoingRoutingKey, message)
	}

	return hashe.WithParseMode(true).Emit(shared.OutgoingRoutingKey, message)
}
