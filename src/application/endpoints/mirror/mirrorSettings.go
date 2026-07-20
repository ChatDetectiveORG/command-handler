package mirror

import (
	"strings"
	"time"

	"github.com/ChatDetectiveORG/command-handler/src/infrastructure/postgresql"
	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	"github.com/gomodule/redigo/redis"
	tele "gopkg.in/telebot.v4"
	redisInfra "github.com/ChatDetectiveORG/command-handler/src/infrastructure/redis"

	models "github.com/ChatDetectiveORG/shared/postgresModels"

	constants "github.com/ChatDetectiveORG/shared/constants"
)

func NewMirrorDeleteEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"mirror_delete",
		*h.HandlerChain{}.Init(
			30*time.Second,
			h.InitChainHandler(runNewMirrorDelete, h.EndOnError),
		),
		h.Or(
			h.UniqueCallback(constants.UniqueMirrorDelete),
			h.UniqueCallback(constants.UniqueMirrorDeleteConfirm),
			h.UniqueCallback(constants.UniqueMirrorDeleteCancel),
		),
	)
	return ep
}

func askApprovement(chatID int64, messageID int, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	message := tele.Message{
		ID: messageID,
		Chat: &tele.Chat{ID: chatID},
		Text: "Вы уверены, что хотите удалить зеркало?",
		ReplyMarkup: &tele.ReplyMarkup{
			InlineKeyboard: [][]tele.InlineButton{
				{{Text: "Да", Data: constants.UniqueMirrorDeleteConfirm}, {Text: "Нет", Data: constants.UniqueMirrorDeleteCancel}},
			},
		},
	}

	return hashe.EmitEditMessage(constants.OutgoingRoutingKey, &message)
}

func mirrorDeleteCancel(chatID int64, messageID int, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	message := tele.Message{
		ID: messageID,
		Chat: &tele.Chat{ID: chatID},
		Text: "Зеркало не будет удалено",
		ReplyMarkup: &tele.ReplyMarkup{
			InlineKeyboard: [][]tele.InlineButton{
				{{Text: "К списку зеркал", Data: constants.UniqueMirrorList}},
			},
		},
	}

	return hashe.EmitEditMessage(constants.OutgoingRoutingKey, &message)
}

func mirrorDeleteConfirm(chatID int64, messageID int, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	db := postgresql.GetDB()

	conn, err := redisInfra.RedisConn()
	if e.IsNonNil(err) {
		return err
	}

	defer conn.Close()

	key := formatMirrorViewHash(chatID)
	mirrorID, eRaw := redis.Int(conn.Do("HGET", key, "ID"))
	if e.IsNonNil(eRaw) {
		return e.Wrap(eRaw)
	}
	_, eRaw = conn.Do("EXPIRE", key, 600)
	if e.IsNonNil(eRaw) {
		return e.Wrap(eRaw)
	}

	mirror, err := models.FindMirrorByID(db, mirrorID)
	if e.IsNonNil(err) {
		return err
	}

	_, eRaw = db.Model(mirror).WherePK().Delete()
	if e.IsNonNil(eRaw) {
		return e.Wrap(eRaw)
	}

	message := tele.Message{
		ID: messageID,
		Chat: &tele.Chat{ID: chatID},
		Text: "Зеркало успешно удалено",
		ReplyMarkup: &tele.ReplyMarkup{
			InlineKeyboard: [][]tele.InlineButton{
				{{Text: "К списку зеркал", Data: constants.UniqueMirrorList}},
			},
		},
	}

	return hashe.EmitEditMessage(constants.OutgoingRoutingKey, &message)
}

func runNewMirrorDelete(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {	
	if strings.HasPrefix(update.Callback.Data, constants.UniqueMirrorDeleteConfirm) {
		return mirrorDeleteConfirm(update.Callback.Message.Chat.ID, update.Callback.Message.ID, hashe)
	}

	if strings.HasPrefix(update.Callback.Data, constants.UniqueMirrorDeleteCancel) {
		return mirrorDeleteCancel(update.Callback.Message.Chat.ID, update.Callback.Message.ID, hashe)
	}

	if strings.HasPrefix(update.Callback.Data, constants.UniqueMirrorDelete) {
		return askApprovement(update.Callback.Message.Chat.ID, update.Callback.Message.ID, hashe)
	}

	return e.NewError("unknown callback data", "failed to run mirror delete endpoint").WithSeverity(e.Notice)
}
