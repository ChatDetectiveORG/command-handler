package help

import (
	"time"

	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	telegram "github.com/ChatDetectiveORG/shared/messageBuilder"
	tele "gopkg.in/telebot.v4"

	constants "github.com/ChatDetectiveORG/shared/constants"
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
	chatID := update.Message.Chat.ID

	mb := telegram.MessageBuilder{Mdv2Enabled: true}

	mb.WriteString("Что умеет этот бот?", telegram.TextFormat{Type: telegram.FormatBold}).
		WriteString("\n\nСписок основных команд:\n").
		WriteString("/check_connection", telegram.TextFormat{Type: telegram.FormatMono}).
		WriteString(" — проверить подключение\n").
		WriteString("/ref", telegram.TextFormat{Type: telegram.FormatMono}).
		WriteString(" — реферальная программа\n").
		WriteString("/export", telegram.TextFormat{Type: telegram.FormatMono}).
		WriteString(" — экспорт чата\n").
		WriteString("/delete_data", telegram.TextFormat{Type: telegram.FormatMono}).
		WriteString(" — удалить данные\n\n").
		WriteString("Не нашли ответ, хотите задать вопрос или сообщить о проблеме? Загляните в @ChatDetectiveSupport.")

	return hashe.WithParseMode(true).Emit(constants.OutgoingRoutingKey, mb.Build(chatID))
}
