package installation

import (
	"context"
	"time"

	shared "github.com/ChatDetectiveORG/command-handler/src/application/endpoints"
	"github.com/ChatDetectiveORG/command-handler/src/infrastructure/postgresql"
	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	models "github.com/ChatDetectiveORG/shared/postgresModels"
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
			h.TextCommand(shared.BtnInstallGuide),
		),
	)
	return ep
}

func run(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	chatID := update.Message.Chat.ID

	if hashe.MirrorID() == "" {
		return hashe.Emit(shared.OutgoingRoutingKey, buildMessage(chatID, tele.File{FileID: shared.InstallationAnimationFileID}))
	}

	mirrorID, err := models.ParseMirrorID(hashe.MirrorID())
	if e.IsNonNil(err) {
		return err
	}
	cachedFileID, err := models.FindMirrorFileID(postgresql.GetDB(), mirrorID, shared.MirrorFileInstallationAnimation)
	if e.IsNonNil(err) {
		return err
	}
	if cachedFileID != "" {
		return hashe.Emit(shared.OutgoingRoutingKey, buildMessage(chatID, tele.File{FileID: cachedFileID}))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	sent, err := hashe.EmitWait(ctx, shared.OutgoingRoutingKey, buildMessage(chatID, tele.FromDisk(shared.InstallationAnimationStaticPath)))
	if e.IsNonNil(err) {
		return err
	}
	if sent != nil && sent.Animation != nil && sent.Animation.FileID != "" {
		return models.UpsertMirrorFileID(postgresql.GetDB(), mirrorID, shared.MirrorFileInstallationAnimation, sent.Animation.FileID, time.Now())
	}
	return e.Nil()
}

func buildMessage(chatID int64, file tele.File) *tele.Message {
	return &tele.Message{
		Chat: &tele.Chat{ID: chatID},
		Animation: &tele.Animation{
			File:     file,
			FileName: "setupInstruction.gif",
			MIME:     "image/gif",
		},
		Caption: "👉Как подключить бота?\n\nСмотрите видео выше или следуйте этим шагам:\n\n1. Зайди в настройки Telegram\n2. Перейди в  раздел \"Telegram для бизнеса\"\n3. Выбери пункт \"Чат-боты\", и в строке поиска введи @ChatDetectiveBot\n4. Нажми на кнопку \"добавить\"",
		CaptionEntities: tele.Entities{
			{Type: tele.EntityCustomEmoji, Offset: 0, Length: 2, CustomEmojiID: "5463392464314315076"},
			{Type: tele.EntityMention, Offset: 196, Length: 17},
		},
	}
}
