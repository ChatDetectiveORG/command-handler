package mirror

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	shared "github.com/ChatDetectiveORG/command-handler/src/application/endpoints"
	"github.com/ChatDetectiveORG/command-handler/src/infrastructure/postgresql"
	paymentservice "github.com/ChatDetectiveORG/payment-service"
	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	models "github.com/ChatDetectiveORG/shared/postgresModels"
	"github.com/ChatDetectiveORG/shared/utils"
	redisInfra "github.com/ChatDetectiveORG/command-handler/src/infrastructure/redis"

	tele "gopkg.in/telebot.v4"
)

func NewMirrorInfoEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"mirror_info",
		*h.HandlerChain{}.Init(
			30*time.Second,
			h.InitChainHandler(runMirrorInfo, h.EndOnError),
		),
		h.UniqueCallback(shared.UniqueMirrorDetails),
	)
	return ep
}

func buildMirrorInfoKeyboard() *tele.ReplyMarkup {
	keyboard := [][]tele.InlineButton{
		{{
			Text: "Удалить зеркало",
			Data: shared.UniqueMirrorDelete,
		}}, {{
			Text: "К списку зеркал",
			Data: shared.UniqueMirrorList,
		}},
	}

	return &tele.ReplyMarkup{
		InlineKeyboard: keyboard,
	}
}

func buildStartMessageText(mirror *models.Mirror) string {
	sb := strings.Builder{}
	sb.WriteString("![🪩](tg://emoji?id=5257960214291823402)*Информация о зеркале " + utils.EscapeMarkdownV2(mirror.BotUsername) + ":*\n\n")

	status := "Онлайн"
	if mirror.Status == models.MirrorStatusPending {
		status = "Ожидает оплаты"
	}

	sb.WriteString("*Статус:* " + status + "\n")
	if mirror.LastUsedAt != nil {
		sb.WriteString("*Время последнего использования:* " + utils.EscapeMarkdownV2(mirror.LastUsedAt.Format("02.01.2006 15:04:05")) + "\n")
	}

	if mirror.PaidUntil != nil {
		sb.WriteString("*Время до оплаты:* " + utils.EscapeMarkdownV2(mirror.PaidUntil.Format("02.01.2006 15:04:05")) + "\n")
		sb.WriteString("*Сумма оплаты:* " + utils.EscapeMarkdownV2(fmt.Sprintf("%d %s", mirror.SourcePayment.Amount, paymentservice.GetCurrencyName(mirror.SourcePayment.Currency))) + "\n")
	}

	return sb.String()
}

func buildMirrorInfoMessage(chatID int64, messageID int, mirror *models.Mirror) *tele.Message {
	return &tele.Message{
		ID:          messageID,
		Chat:        &tele.Chat{ID: chatID},
		Text:        buildStartMessageText(mirror),
		ReplyMarkup: buildMirrorInfoKeyboard(),
	}
}

func runMirrorInfo(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	if update.Callback == nil {
		return e.NewError("callback is nil", "failed to run mirror info endpoint").WithSeverity(e.Notice)
	}

	var callbackData map[string]any
	err := e.Wrap(json.Unmarshal([]byte(strings.TrimPrefix(update.Callback.Data, shared.UniqueMirrorDetails)), &callbackData))
	if e.IsNonNil(err) {
		return err
	}

	mirrorIDFloat, ok := callbackData["mirror_id"].(float64)
	if !ok {
		return e.NewError("invalid mirror_id type", "failed to convert mirror_id to float64").WithSeverity(e.Notice)
	}
	mirrorIDInt := int(mirrorIDFloat)

	db := postgresql.GetDB()
	mirror, err := models.FindMirrorByID(db, mirrorIDInt)
	if e.IsNonNil(err) {
		return err
	}

	msg := buildMirrorInfoMessage(update.Callback.Message.Chat.ID, update.Callback.Message.ID, mirror)
	if msg == nil {
		return e.NewError("failed to build mirror info message", "failed to run mirror info endpoint").WithSeverity(e.Notice)
	}

	hash := formatMirrorViewHash(update.Callback.Message.Chat.ID)
	conn, err := redisInfra.RedisConn()
	if e.IsNonNil(err) {
		return err
	}
	defer conn.Close()
	_, eRaw := conn.Do("HSET", hash, "ID", mirrorIDInt)
	if e.IsNonNil(eRaw) {
		return e.Wrap(eRaw)
	}
	_, eRaw = conn.Do("EXPIRE", hash, 600)
	if e.IsNonNil(eRaw) {
		return e.Wrap(eRaw)
	}

	return hashe.WithParseMode(true).EmitEditMessage(shared.OutgoingRoutingKey, msg)
}

func formatMirrorViewHash(chatID int64) string {
	return fmt.Sprintf("mirror:view:%d", chatID)
}
