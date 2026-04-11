package endpoints

import (
	"app/src/infrastructure/postgresql"
	"context"
	"time"

	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	models "github.com/ChatDetectiveORG/shared/postgresModels"
	tele "gopkg.in/telebot.v4"
)

func StartCommand() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"save",
		*h.HandlerChain{}.Init(
			10 * time.Second,
			h.InitChainHandler(greet, h.EndOnError),
		),
		h.Or(h.Command([]string{"start"}), h.TextCommand("Старт")),
	)

	return ep
}

func greet(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	tx, eraw := postgresql.GetDB().Begin()
	if e.IsNonNil(eraw) {
		return e.FromError(eraw, "failed to begin transaction")
	}
	err := (&models.Telegramuser{}).GetOrCreate(tx, update.Message.Sender)
	if e.IsNonNil(err) {
		return e.FromError(err, "failed to get or create telegram user")
	}
	eraw = tx.Commit()
	if e.IsNonNil(eraw) {
		return e.FromError(eraw, "failed to commit transaction")
	}
	eraw = tx.Close()
	if e.IsNonNil(eraw) {
		return e.FromError(eraw, "failed to close transaction")
	}

	err = hashe.Emit("telegram.message.send", &tele.Message{
		Chat: &tele.Chat{
			ID: update.Message.Chat.ID,
		},
		Text: "Command Success",
	})
	if e.IsNonNil(err) {
		return err
	}

	msg, err := hashe.EmitWait(context.Background(), "telegram.message.send", &tele.Message{
		Chat: &tele.Chat{
			ID: update.Message.Chat.ID,
		},
		Text: "Message To Reply",
	})
	if e.IsNonNil(err) {
		return err
	}

	err = hashe.Emit("telegram.message.send", &tele.Message{
		Chat: &tele.Chat{
			ID: update.Message.Chat.ID,
		},
		Text: "Reply",
		ReplyTo: msg,
	})
	if e.IsNonNil(err) {
		return err
	}

	return e.Nil()
}
