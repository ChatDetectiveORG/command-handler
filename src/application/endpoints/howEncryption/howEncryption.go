package howencryption

import (
	"time"

	shared "github.com/ChatDetectiveORG/command-handler/src/application/endpoints"
	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	tele "gopkg.in/telebot.v4"
)

func NewHowEncryptionEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"how_encryption",
		*h.HandlerChain{}.Init(
			30*time.Second,
			h.InitChainHandler(run, h.EndOnError),
		),
		h.TextCommand(shared.BtnHowEncryption),
	)
	return ep
}

func run(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	msg := &tele.Message{
		Chat:  &tele.Chat{ID: update.Message.Chat.ID},
		Photo: &tele.Photo{File: tele.File{FileID: shared.HowEncryptionPhotoFileID}},
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
	return hashe.Emit(shared.OutgoingRoutingKey, msg)
}
