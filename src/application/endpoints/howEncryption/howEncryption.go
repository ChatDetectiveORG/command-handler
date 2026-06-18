package howencryption

import (
	"context"
	"time"

	"github.com/ChatDetectiveORG/command-handler/src/infrastructure/postgresql"
	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	"github.com/ChatDetectiveORG/shared/telegram"
	tele "gopkg.in/telebot.v4"

	constants "github.com/ChatDetectiveORG/shared/constants"
)

func NewHowEncryptionEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"how_encryption",
		*h.HandlerChain{}.Init(
			2*time.Minute,
			h.InitChainHandler(run, h.EndOnError),
		),
		h.TextCommand(constants.BtnHowEncryption),
	)
	return ep
}

func run(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	chatID := update.Message.Chat.ID
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	return hashe.WithParseMode(true).EmitBuilt(
		ctx,
		postgresql.GetDB(),
		constants.OutgoingRoutingKey,
		chatID,
		buildMessage(),
	)
}

func buildMessage() *telegram.MessageBuilder {
	mb := &telegram.MessageBuilder{Mdv2Enabled: true}
	mb.WriteString("Твоя приватность защищена на уровне архитектуры", telegram.TextFormat{Type: telegram.Bold}).
		CustomeEmoji("🔓", "5375174248371347954").
		WriteNextLine("\n\nВсе данные хранятся в полностью зашифрованном и деперсонализированном виде. Мы внедрили систему, которая исключает человеческий фактор:").
		WriteNextLine("🐾", telegram.TextFormat{Type: telegram.Link}.WithCustomEmojiID("6062215036858797217")).
		WriteNextLine("Многослойное шифрование: При регистрации тебе создается персональный ключ. Он сам зашифрован общим мастер-ключом, который хранится в изолированной среде.").WriteString("🐾", telegram.TextFormat{Type: telegram.Link}.WithCustomEmojiID("6062215036858797217")).WriteString(
		"Нулевой доступ: У администрации и сторонних сервисов нет технической возможности прочитать твои данные. Даже по запросу силовых структур мы не сможем выдать переписку — у нас просто нет ключей для её расшифровки.",
	).WriteString("\n\nТем временем на фото выше представлены данные администратора бота").WriteString(
		"👩‍💻", telegram.TextFormat{Type: telegram.Link}.WithCustomEmojiID("6325758868706035396"),
	)
	mb.AddMirrorFile(telegram.MirrorFileAsset{
		PrimaryFileID: constants.HowEncryptionPhotoFileID,
		FallbackPath:  constants.HowEncryptionPhotoStaticPath,
		MimeType:      "image/png",
		MirrorFileKey: constants.MirrorFileHowEncryptionPhoto,
	})

	return mb
}
