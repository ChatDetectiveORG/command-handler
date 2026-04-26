package help

import (
	"time"

	shared "github.com/ChatDetectiveORG/command-handler/src/application/endpoints"
	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	tele "gopkg.in/telebot.v4"
)

func NewHelpEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"help",
		*h.HandlerChain{}.Init(
			30*time.Second,
			h.InitChainHandler(run, h.EndOnError),
		),
		h.Command([]string{"help"}),
	)
	return ep
}

func run(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	msg := &tele.Message{
		Chat: &tele.Chat{ID: update.Message.Chat.ID},
		Text: "🔓Что умеет этот бот?\n\nСписок команд:\n😵 /bug — Сообщить о проблеме\n🙈/check_connection — Проверить подключение\n🛍/ref — Реферальная программа\n\nПоддержка бота: @ChatDetectiveSupport",
		Entities: tele.Entities{
			{Type: tele.EntityCustomEmoji, Offset: 0, Length: 2, CustomEmojiID: "5465443379917629504"},
			{Type: tele.EntityCustomEmoji, Offset: 38, Length: 2, CustomEmojiID: "5465265370703080100"},
			{Type: tele.EntityCommand, Offset: 41, Length: 4},
			{Type: tele.EntityCustomEmoji, Offset: 68, Length: 2, CustomEmojiID: "5463345378587849154"},
			{Type: tele.EntityCommand, Offset: 70, Length: 17},
			{Type: tele.EntityCustomEmoji, Offset: 112, Length: 2, CustomEmojiID: "5453901475648390219"},
			{Type: tele.EntityCommand, Offset: 114, Length: 4},
			{Type: tele.EntityMention, Offset: 160, Length: 21},
		},
	}
	return hashe.Emit(shared.OutgoingRoutingKey, msg)
}
