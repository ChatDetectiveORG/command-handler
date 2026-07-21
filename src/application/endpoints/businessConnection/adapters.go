package businessconnection

import (
	"time"

	"github.com/ChatDetectiveORG/command-handler/src/infrastructure/postgresql"
	"github.com/ChatDetectiveORG/shared/constants"
	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	postgresmodels "github.com/ChatDetectiveORG/shared/postgresModels"
	utils "github.com/ChatDetectiveORG/shared/utils"
	tele "gopkg.in/telebot.v4"

	commandhandlerutils "github.com/ChatDetectiveORG/shared/commandHandlerUtils"
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

	user, err := commandhandlerutils.GetUserByTgID(db, conn.Sender.ID)
	if e.IsNonNil(err) {
		return err
	}

	if conn.Enabled {
		previousHash := user.BusinessConnectionIDHash
		newHash, hashErr := utils.ToSecureHash(conn.ID)
		if e.IsNonNil(hashErr) {
			return hashErr
		}

		if previousHash != "" {
			updatedFields := &postgresmodels.Message{
				BusinessConnectionIDHash: newHash,
			}
			_, eRaw := db.Model(updatedFields).
				Column("business_connection_id_hash").
				Where("business_connection_id_hash = ?", previousHash).
				Update()
			if e.IsNonNil(eRaw) {
				return e.FromError(eRaw, "failed to update business connection id hash").WithSeverity(e.Notice)
			}
		}

		if err := commandhandlerutils.SetBusinessConnectionConnected(db, user, conn.ID); e.IsNonNil(err) {
			return err
		}

		if err := hashe.WithParseMode(true).Emit(constants.OutgoingRoutingKey, buildConnectedMessage(userChatID)); e.IsNonNil(err) {
			return err
		}

		err = referralMain(db, user, hashe, true)
		if e.IsNonNil(err) {
			return err
		}
	} else {
		if err := commandhandlerutils.SetBusinessConnectionDisconnected(db, user); e.IsNonNil(err) {
			return err
		}

		disconnectedMsg := buildDisconnectedMessage(userChatID)
		if err := hashe.WithParseMode(true).Emit(constants.OutgoingRoutingKey, disconnectedMsg); e.IsNonNil(err) {
			return err
		}

		err = referralMain(db, user, hashe, false)
		if e.IsNonNil(err) {
			return err
		}
	}

	return e.Nil()
}
