package businessconnection

import (
	"time"

	"github.com/ChatDetectiveORG/command-handler/src/infrastructure/postgresql"
	h "github.com/ChatDetectiveORG/shared/handlers"
	tele "gopkg.in/telebot.v4"
	e "github.com/ChatDetectiveORG/shared/errors"
	shared "github.com/ChatDetectiveORG/command-handler/src/application/endpoints"
)

// Notify user about cases when bot connected or disconnected
//
// Handles referral relations
//
// Takes: update, hashe
//
// Returns: error
func NewBusinessConnectionEndpoint() h.Endpoint {
	ep := h.Endpoint{}
	ep.Init(
		"business_connection",
		*h.HandlerChain{}.Init(
			2*time.Minute,
			h.InitChainHandler(run, h.EndOnError),
		),
		h.BusinessConnectionChanged(),
	)
	return ep
}

func run(update tele.Update, hashe *h.HandlerChainHashe) *e.ErrorInfo {
	db := postgresql.GetDB()
	conn := update.BusinessConnection
	userChatID := conn.UserChatID

	user, err := shared.GetUserByTgID(db, conn.Sender.ID)
	if e.IsNonNil(err) {
		return err
	}

	if conn.Enabled {
		if err := shared.UpdateBusinessConnectionIDHash(db, user, conn.ID); e.IsNonNil(err) {
			return err
		}

		if err := hashe.WithParseMode(true).Emit(shared.OutgoingRoutingKey, buildConnectedMessage(userChatID)); e.IsNonNil(err) {
			return err
		}

		err = referralMain(db, user, hashe, true)
		if e.IsNonNil(err) {
			return err
		}
	} else {
		if err := shared.UpdateBusinessConnectionIDHash(db, user, ""); e.IsNonNil(err) {
			return err
		}

		disconnectedMsg := buildDisconnectedMessage(userChatID)
		if err := hashe.WithParseMode(true).Emit(shared.OutgoingRoutingKey, disconnectedMsg); e.IsNonNil(err) {
			return err
		}

		err = referralMain(db, user, hashe, false)
		if e.IsNonNil(err) {
			return err
		}
	}

	return e.Nil()
}