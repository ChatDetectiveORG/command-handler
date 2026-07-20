package start

import (
	"time"

	"github.com/ChatDetectiveORG/command-handler/src/infrastructure/postgresql"
	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	"github.com/ChatDetectiveORG/shared/legal"
	models "github.com/ChatDetectiveORG/shared/postgresModels"
	utils "github.com/ChatDetectiveORG/shared/utils"
	tele "gopkg.in/telebot.v4"

	helpers "github.com/ChatDetectiveORG/shared/commandHandlerUtils"
	constants "github.com/ChatDetectiveORG/shared/constants"
)

// NewLegalConsentEndpoint handles the "Принимаю условия" inline button.
func NewLegalConsentEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"legal_consent",
		*h.HandlerChain{}.Init(
			30*time.Second,
			h.InitChainHandler(runLegalConsent, h.EndOnError),
		),
		h.UniqueCallback(constants.UniqueLegalConsent),
	)
	return ep
}

// needsConsentGate reports whether the user must accept the current legal docs version
// before getting access to the main menu. When legal docs are not configured (dev
// environments) the gate is disabled; production values are enforced via Helm.
func needsConsentGate(user *models.Telegramuser, docs legal.Docs) (bool, *e.ErrorInfo) {
	if !docs.Configured() {
		return false, e.Nil()
	}
	hasConsent, err := models.HasUserConsent(postgresql.GetDB(), user.ID, docs.Version)
	if e.IsNonNil(err) {
		return false, err
	}
	return !hasConsent, e.Nil()
}

// buildConsentMessage shows the legal document links and an explicit accept button.
func buildConsentMessage(docs legal.Docs, chatID int64) *tele.Message {
	var text string
	var entities tele.Entities

	appendLine := func(line string) {
		text += line
	}
	appendLink := func(prefix, label, url string) {
		appendLine(prefix)
		offset := utils.TgLen(text)
		appendLine(label)
		entities = append(entities, tele.MessageEntity{
			Type:   tele.EntityTextLink,
			Offset: offset,
			Length: utils.TgLen(label),
			URL:    url,
		})
	}

	appendLine("Прежде чем начать пользоваться ботом, ознакомься с документами и подтверди согласие:\n\n")
	appendLink("• ", "Пользовательское соглашение", docs.AgreementURL)
	appendLink("\n• ", "Политика обработки персональных данных", docs.PrivacyURL)
	appendLink("\n• ", "Согласие на обработку персональных данных", docs.ConsentURL)
	appendLine("\n\nНажимая «Принимаю условия», ты подтверждаешь, что ознакомился с документами (версия " + docs.Version + ") и принимаешь их.")

	return &tele.Message{
		Chat:           &tele.Chat{ID: chatID},
		Text:           text,
		Entities:       entities,
		PreviewOptions: &tele.PreviewOptions{Disabled: true},
		ReplyMarkup: &tele.ReplyMarkup{
			InlineKeyboard: [][]tele.InlineButton{
				{{Text: "✅ Принимаю условия", Data: constants.UniqueLegalConsent}},
			},
		},
	}
}

func runLegalConsent(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	db := postgresql.GetDB()
	docs := legal.FromEnv()

	user, err := helpers.GetUserByTgID(db, update.Callback.Sender.ID)
	if e.IsNonNil(err) {
		return err.PushStack()
	}

	if docs.Configured() {
		if err := models.RecordUserConsent(db, user.ID, docs.Version, legal.ConsentSourceStart, time.Now()); e.IsNonNil(err) {
			return err.PushStack()
		}
	}

	if err := hashe.EmitCallback(
		constants.OutgoingRoutingKey,
		update.Callback,
		helpers.AnswerCallbackBanner("Спасибо! Согласие сохранено.", update.Callback),
	); e.IsNonNil(err) {
		return err.PushStack()
	}

	// After consent, show the regular welcome message with the main menu.
	return hashe.Emit(
		constants.OutgoingRoutingKey,
		buildWelcomeMessage(update.Callback.Sender, update.Callback.Message.Chat.ID),
	)
}
