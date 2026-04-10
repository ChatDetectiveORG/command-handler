package endpoints

import (
	"context"
	"time"

	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
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
	err := hashe.Emit("telegram.message.send", &tele.Message{
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
