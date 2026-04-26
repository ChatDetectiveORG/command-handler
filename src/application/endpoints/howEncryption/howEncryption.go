package howencryption

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

func NewHowEncryptionEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"how_encryption",
		*h.HandlerChain{}.Init(
			2*time.Minute,
			h.InitChainHandler(run, h.EndOnError),
		),
		h.TextCommand(shared.BtnHowEncryption),
	)
	return ep
}

func run(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	chatID := update.Message.Chat.ID
	if hashe.MirrorID() == "" {
		return hashe.Emit(shared.OutgoingRoutingKey, buildMessage(chatID, tele.File{FileID: shared.HowEncryptionPhotoFileID}))
	}

	mirrorID, err := models.ParseMirrorID(hashe.MirrorID())
	if e.IsNonNil(err) {
		return err
	}
	cachedFileID, err := models.FindMirrorFileID(postgresql.GetDB(), mirrorID, shared.MirrorFileHowEncryptionPhoto)
	if e.IsNonNil(err) {
		return err
	}
	if cachedFileID != "" {
		return hashe.Emit(shared.OutgoingRoutingKey, buildMessage(chatID, tele.File{FileID: cachedFileID}))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	sent, err := hashe.EmitWait(ctx, shared.OutgoingRoutingKey, buildMessage(chatID, tele.FromDisk(shared.HowEncryptionPhotoStaticPath)))
	if e.IsNonNil(err) {
		return err
	}
	if sent != nil && sent.Photo != nil && sent.Photo.FileID != "" {
		return models.UpsertMirrorFileID(postgresql.GetDB(), mirrorID, shared.MirrorFileHowEncryptionPhoto, sent.Photo.FileID, time.Now())
	}
	return e.Nil()
}

func buildMessage(chatID int64, file tele.File) *tele.Message {
	return &tele.Message{
		Chat:    &tele.Chat{ID: chatID},
		Photo:   &tele.Photo{File: file},
		Caption: "Твоя приватность защищена на уровне архитектуры🔓\n\nВсе данные хранятся в полностью зашифрованном и деперсонализированном виде. Мы внедрили систему, которая исключает человеческий фактор:\n🐾 Многослойное шифрование: При регистрации тебе создается персональный ключ. Он сам зашифрован общим мастер-ключом, который хранится в изолированной среде.\n🐾 Нулевой доступ: У администрации и сторонних сервисов нет технической возможности прочитать твои данные. Даже по запросу силовых структур мы не сможем выдать переписку — у нас просто нет ключей для её расшифровки.\n🐾 Безопасная обработка: Все операции проходят в доверенной среде исполнения (TEE). Это значит, что ключи не видны даже администратору с полным доступом к серверу.\n\nТем временем на фото выше представлены данные администратора бота👩‍💻",
		CaptionEntities: tele.Entities{
			{Type: tele.EntityBold, Offset: 0, Length: 47},
			{Type: tele.EntityBold, Offset: 47, Length: 2},
			{Type: tele.EntityCustomEmoji, Offset: 47, Length: 2, CustomEmojiID: "5375174248371347954"},
			{Type: tele.EntityCustomEmoji, Offset: 187, Length: 2, CustomEmojiID: "6062215036858797217"},
			{Type: tele.EntityCustomEmoji, Offset: 344, Length: 2, CustomEmojiID: "6062215036858797217"},
			{Type: tele.EntityCustomEmoji, Offset: 560, Length: 2, CustomEmojiID: "6062215036858797217"},
			{Type: tele.EntityCustomEmoji, Offset: 790, Length: 5, CustomEmojiID: "6325758868706035396"},
		},
	}
}
