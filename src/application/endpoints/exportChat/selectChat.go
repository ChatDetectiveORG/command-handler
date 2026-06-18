package exportchat

import (
	"time"

	"github.com/ChatDetectiveORG/command-handler/src/infrastructure/postgresql"
	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	models "github.com/ChatDetectiveORG/shared/postgresModels"
	"github.com/ChatDetectiveORG/shared/telegram"
	"github.com/ChatDetectiveORG/shared/utils"
	tele "gopkg.in/telebot.v4"

	cdredis "github.com/ChatDetectiveORG/command-handler/src/infrastructure/redis"

	helpers "github.com/ChatDetectiveORG/shared/commandHandlerUtils"
	constants "github.com/ChatDetectiveORG/shared/constants"
)

func NewSelectChatEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"show_contacts",
		*h.HandlerChain{}.Init(
			30*time.Second,
			h.InitChainHandler(runShowContacts, h.EndOnError),
		),
		h.Or(h.Command([]string{"export"}), h.TextCommand("Экспорт чата"), h.CallbackStartsWith(constants.UniqueChatSelectPage)),
	)
	return ep
}

func runShowContacts(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	db := postgresql.GetDB()

	messageBuilder := telegram.MessageBuilder{Mdv2Enabled: true}
	messageBuilder.WriteString(
		"Выберите чат для восстановления:",
	)

	var senderID int64
	var chatID int64
	if update.Callback != nil {
		senderID = update.Callback.Sender.ID
		chatID = update.Callback.Message.Chat.ID
	} else {
		senderID = update.Message.Sender.ID
		chatID = update.Message.Chat.ID
	}

	sender, err := helpers.GetUserByTgID(db, senderID)
	if e.IsNonNil(err) {
		return err.PushStack()
	}

	redisConn, err := cdredis.RedisConn()
	if e.IsNonNil(err) {
		return err.PushStack()
	}
	defer redisConn.Close()

	var data string = ""
	if update.Callback != nil {
		data = update.Callback.Data
	}
	
	telegram.CreateGenericKeyboard[*models.Telegramuser](
		&messageBuilder,
		db.Model(&models.Telegramuser{}).
			Join("JOIN user_relations AS r ON (r.first_user_id = telegramuser.id AND r.second_user_id = ?) OR (r.second_user_id = telegramuser.id AND r.first_user_id = ?)", sender.ID, sender.ID).
			Where("telegramuser.id != ?", sender.ID).
			Order("telegramuser.created_at ASC"),
		redisConn,
		db,
		data,
		telegram.CreateGenericKeyboardParams{
			ChatID: chatID,
			PageUnique: constants.UniqueChatSelectPage,
			ButtonsPerPage: 7,
			ButtonsPerRow: 1,
			ShowNavigation: true,
			ButtonConversionArgs: telegram.TelegramButtonConversionArgs{
				AdditionalData: map[string]any{
					"userBusinessConnectionIdHash": sender.BusinessConnectionIDHash,
				},
				CallbackDataProducer: func(userRefID string) string {
					res := utils.DumpCallbackData(constants.UniqueGoToChat, map[string]any{constants.CallbackFieldCode: userRefID})
					return res
				},
			},
		},
	)

	message := messageBuilder.Build(chatID)
	if update.Callback != nil {
		message.ID = update.Callback.Message.ID
	}

	if update.Callback == nil {
		err = hashe.Emit(constants.OutgoingRoutingKey, message)

		return err
	}

	err = hashe.EmitEditMessage(constants.OutgoingRoutingKey, message)

	return err
}
