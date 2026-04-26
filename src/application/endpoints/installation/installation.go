package installation

import (
	"time"

	shared "github.com/ChatDetectiveORG/command-handler/src/application/endpoints"
	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	tele "gopkg.in/telebot.v4"
)

func NewInstallationEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"installation",
		*h.HandlerChain{}.Init(
			30*time.Second,
			h.InitChainHandler(run, h.EndOnError),
		),
		h.Or(
			h.Command([]string{"install"}),
			h.TextCommand(shared.BtnInstallGuide),
		),
	)
	return ep
}

func run(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	chatID := update.Message.Chat.ID

	msg := &tele.Message{
		Chat: &tele.Chat{ID: chatID},
		Animation: &tele.Animation{
			File: tele.File{FileID: shared.InstallationAnimationFileID},
		},
		Caption: "👉Как подключить бота?\n\nСмотрите видео выше или следуйте этим шагам:\n\n1. Зайди в настройки Telegram\n2. Перейди в  раздел \"Telegram для бизнеса\"\n3. Выбери пункт \"Чат-боты\", и в строке поиска введи @ChatDetectiveBot\n4. Нажми на кнопку \"добавить\"",
		CaptionEntities: tele.Entities{
			{Type: tele.EntityCustomEmoji, Offset: 0, Length: 2, CustomEmojiID: "5463392464314315076"},
			{Type: tele.EntityMention, Offset: 196, Length: 17},
		},
	}

	return hashe.Emit(shared.OutgoingRoutingKey, msg)
}
