package installation

import (
	"context"
	"time"

	"github.com/ChatDetectiveORG/command-handler/src/infrastructure/postgresql"
	constants "github.com/ChatDetectiveORG/shared/constants"
	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	telegram "github.com/ChatDetectiveORG/shared/messageBuilder"
	tele "gopkg.in/telebot.v4"
)

func NewInstallationEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"installation",
		*h.HandlerChain{}.Init(
			2*time.Minute,
			h.InitChainHandler(run, h.EndOnError),
		),
		h.Or(
			h.Command([]string{"install"}),
			h.TextCommand(constants.BtnInstallGuide),
		),
	)
	return ep
}

func run(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	chatID := update.Message.Chat.ID
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	return hashe.EmitBuilt(
		ctx,
		postgresql.GetDB(),
		constants.OutgoingRoutingKey,
		chatID,
		buildMessage(),
	)
}

func buildMessage() *telegram.MessageBuilder {
	mb := &telegram.MessageBuilder{}
	mb.WriteString("👉", telegram.TextFormat{Type: telegram.FormatLink}.WithCustomEmojiID("5463392464314315076")).WriteString(
		"Как подключить бота?\n\nСмотрите видео выше или следуйте этим шагам:\n\n1. Зайди в настройки Telegram\n2. Перейди в  раздел \"Telegram для бизнеса\"\n3. Выбери пункт \"Чат-боты\", и в строке поиска введи ",
	).WriteString(
		"@ChatDetectiveBot",
	).WriteString(
		"\n4. Нажми на кнопку \"добавить\"",
	)
	mb.AddMirrorFile(telegram.MirrorFileAsset{
		PrimaryFileID: constants.InstallationAnimationFileID,
		FallbackPath:  constants.InstallationAnimationStaticPath,
		MimeType:      "image/gif",
		MirrorFileKey: constants.MirrorFileInstallationAnimation,
	})

	return mb
}
