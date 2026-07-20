package businessconnection

import (
	"errors"
	"strconv"
	"time"

	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	. "github.com/ChatDetectiveORG/shared/messageBuilder"
	models "github.com/ChatDetectiveORG/shared/postgresModels"
	"github.com/go-pg/pg/v10"
	tele "gopkg.in/telebot.v4"

	constants "github.com/ChatDetectiveORG/shared/constants"
	levelmanagement "github.com/ChatDetectiveORG/shared/levelManagement"
)

// Gets used link owner and referral relation between users
// Takse: transaction, actor (invited user)
// Returns: referral, invitor (link owner), error
func getReferralAndLinkOwner(tx *pg.Tx, actor *models.Telegramuser) (*models.Referral, *models.Telegramuser, *e.ErrorInfo) {
	referral := &models.Referral{}
	err := e.Wrap(tx.Model(referral).Where("invited_user_id = ?", actor.ID).Select())
	if e.IsNonNil(err) {
		if errors.Is(err.Err, pg.ErrNoRows) {
			return nil, nil, err.WithSeverity(e.Ingnored)
		}

		return nil, nil, err
	}

	invitor := &models.Telegramuser{
		ID: referral.InvitorID,
	}
	err = e.Wrap(tx.Model(invitor).WherePK().Relation("Settings").Select())
	if e.IsNonNil(err) {
		return nil, nil, err
	}

	return referral, invitor, e.Nil()
}

// Applies or deletes referral bonuses for link owner
// Takes: database, actor (invited user), connected (true if invited user connected bot)
func updateReferral(db *pg.DB, actor *models.Telegramuser, connected bool) *e.ErrorInfo {
	tx, eraw := db.Begin()
	if eraw != nil {
		return e.FromError(eraw, "failed to begin transaction").WithSeverity(e.Critical)
	}
	defer tx.Rollback()

	referral, invitor, err := getReferralAndLinkOwner(tx, actor)
	if e.IsNonNil(err) {
		return err
	}

	if invitor.Settings == nil {
		return e.NewError("user_settings_not_found", "user settings not found").WithSeverity(e.Ingnored)
	}

	referral.FixedRewardType = invitor.Settings.ReferralBonusPreference

	switch invitor.Settings.ReferralBonusPreference {
	case models.ReferralBonusMoney:
		referral.FixedMoneyReward = constants.ReferralBonusRub
	case models.ReferralBonusLevels:
		referral.ActualUntil = time.Now().Add(time.Duration(constants.ReferralLevelsDurationSec) * time.Second)
	}

	referral.UpdatedAt = time.Now()

	if !connected {
		_, eRaw := tx.Model(referral).WherePK().Delete()
		if e.IsNonNil(eRaw) {
			return e.Wrap(eRaw)
		}

		if eraw = tx.Commit(); eraw != nil {
			return e.FromError(eraw, "failed to commit transaction").WithSeverity(e.Critical)
		}

		return e.Nil()
	}

	_, eRaw := tx.Model(referral).WherePK().Column("actual_until", "fixed_money_reward", "fixed_reward_type", "updated_at").Update()
	if eRaw != nil {
		return e.FromError(eRaw, "failed to update referral").WithSeverity(e.Critical)
	}

	if eraw = tx.Commit(); eraw != nil {
		return e.FromError(eraw, "failed to commit transaction").WithSeverity(e.Critical)
	}

	return e.Nil()
}

// Handles levels bonus for invitor (adds or removes levels based on connected status)
// Takes: transaction, referral, message builder, connected (true if invited user connected bot)
// Returns: error
func handleLevels(tx *pg.Tx, referral *models.Referral, messageBuilder *MessageBuilder, connected bool) *e.ErrorInfo {
	invitorID := referral.InvitorID

	levelSummary, err := models.GetUserLevelSummary(tx, invitorID, time.Now())
	if e.IsNonNil(err) {
		return err
	}

	if !connected {
		var addictedBonus models.UserLevels
		err := e.Wrap(tx.Model(&addictedBonus).
			Where("linked_user_id = ?", invitorID).
			Where("? = ANY(linked_referral_ids)", referral.ID).
			Limit(1).
			Select(),
		)
		if e.IsNonNil(err) && !errors.Is(err.Err, pg.ErrNoRows) {
			return err
		}
		if addictedBonus.ID == 0 {
			return e.Nil()
		}

		_, eRaw := tx.Model(&addictedBonus).WherePK().Delete()
		if e.IsNonNil(eRaw) {
			return e.Wrap(eRaw)
		}

		_, eRaw = tx.Model(&models.Referral{}).
			Where("id = ?", referral.ID).
			Delete()
		if e.IsNonNil(eRaw) {
			return e.Wrap(eRaw)
		}

		untrackedRalations, err := levelmanagement.GetUntrackedRelations(tx, invitorID)
		if e.IsNonNil(err) {
			return err
		}

		levelsAdded, err := levelmanagement.RecountLevels(tx, untrackedRalations, invitorID)
		if e.IsNonNil(err) {
			return err
		}

		if levelsAdded == 0 {
			messageBuilder.Write(
				E("5462882007451185227", "🚫"),
				T("Твой уровень снижен на 1."),
				T("Сейчас он составляет: %d", Args{A: []any{levelSummary.Level - 1}}),
			)
		} else {
			messageBuilder.Write(
				E("5256047523620995497", "🔥"),
				T("Твой уровень поднят на %d.", Args{A: []any{levelsAdded}}),
				T("Сейчас он составляет: %d", Args{A: []any{levelSummary.Level + levelsAdded}}),
			)
		}

		return e.Nil()
	}

	err = e.Wrap(tx.Model(&models.UserLevels{}).
		Where("linked_user_id = ?", invitorID).
		Where("? = ANY(linked_referral_ids)", referral.ID).
		Select(),
	)
	if e.IsNonNil(err) && !errors.Is(err.Err, pg.ErrNoRows) {
		return err
	}
	if e.IsNil(err) && connected {
		return e.NewError("referral already considered", "referral already considered").WithSeverity(e.Ingnored)
	}

	untrackedRalations, err := levelmanagement.GetUntrackedRelations(tx, invitorID)
	if e.IsNonNil(err) {
		return err
	}

	levelsAdded, err := levelmanagement.RecountLevels(tx, untrackedRalations, invitorID)
	if e.IsNonNil(err) {
		return err
	}

	userNumber := constants.ReferralBonusThresholdLevels - (len(untrackedRalations) - levelsAdded*constants.ReferralBonusThresholdLevels)

	if levelsAdded == 0 {
		messageBuilder.Write(
			E("5256047523620995497", "🔥"),
			T("До поднятия уровня осталось привести %d пользователей!", Args{A: []any{userNumber}}),
			T("Сейчас он составляет: %d", Args{A: []any{levelSummary.Level}}),
		)
	} else {
		messageBuilder.Write(
			T("Твой уровень сейчас составляет "+strconv.Itoa(levelSummary.Level+levelsAdded)),
			T(""),
			T("Ты получил уровней: %d", Args{A: []any{levelsAdded}}),
			T("Приведи %d пользователей чтобы получить ещё один бонусный уровень!", Args{A: []any{userNumber}}),
		)
	}

	return e.Nil()
}

// Writes removed or added money bonus text
// Takes: message builder, connected (true if invited user connected bot)
func handleMoney(messageBuilder *MessageBuilder, connected bool) {
	if connected {
		creditDate := time.Now().Add(time.Duration(constants.ReferralDiscountDurationSec)*time.Second + time.Hour*24).Format("02.01.2006")
		messageBuilder.Write(
			E("5256047523620995497", "🔥"),
			T("Если он не будет отключать бота до %s, ты получишь %d рублей на внутренний счёт бота", Args{A: []any{creditDate, constants.ReferralBonusRub}}),
		)

		return
	}

	duration := time.Now().Add(time.Duration(constants.ReferralDiscountDurationSec)*time.Second + time.Hour*24).Format("02.01.2006")
	messageBuilder.Write(
		E("5462882007451185227", "🚫"),
		T("Ты не получишь выплат за него"),
		T("Чтобы получить денги за него, он должен подключить бота и пользоваться им без перерыва в течение %.0f", Args{A: []any{duration}}),
	)
}

// Sends all needed referral messages
// Takes: database, invited user, hashe, connection status(true if invited user connected bot)
func sendReferral(db *pg.DB, actor *models.Telegramuser, hashe *h.HandlerChainHashe, connected bool) *e.ErrorInfo {
	tx, eraw := db.Begin()
	if eraw != nil {
		return e.FromError(eraw, "failed to begin transaction").WithSeverity(e.Critical)
	}
	defer tx.Rollback()

	referral, invitor, err := getReferralAndLinkOwner(tx, actor)
	if e.IsNonNil(err) {
		return err
	}

	if invitor.Settings == nil {
		return e.NewError("user_settings_not_found", "user settings not found").WithSeverity(e.Ingnored)
	}

	messageBuilder := MessageBuilder{Mdv2Enabled: true}
	messageBuilder.Write(
		E("5463071033256848094"),
	)

	actorFullName, err := actor.GetFullName()
	if e.IsNonNil(err) {
		return err
	}
	actorTgID, err := actor.GetTgId()
	if e.IsNonNil(err) {
		return err
	}

	messageBuilder.Write(
		T("Пользователь ", Args{NoNewline: true}),
		UserMention(actorFullName, actorTgID), T(" ", Args{NoNewline: true}),
	)

	if connected {
		messageBuilder.Write(
			T("подключил бота!"),
			T(""),
		)
	} else {
		messageBuilder.Write(
			T("отключил бота!"),
			T(""),
		)
	}

	switch invitor.Settings.ReferralBonusPreference {
	case models.ReferralBonusMoney:
		handleMoney(&messageBuilder, connected)

		messageBuilder.AddButton(
			tele.InlineButton{Text: "Посмотреть баланс", Data: "-"},
		).AddButton(
			tele.InlineButton{Text: "Подробнее", Data: "-"},
		).NextRow()
	case models.ReferralBonusLevels:
		err = handleLevels(tx, referral, &messageBuilder, connected)
		if e.IsNonNil(err) {
			return err
		}

		messageBuilder.AddButton(tele.InlineButton{Text: "Посмотреть уровни", Data: "-"}).NextRow()
	}

	invitorTgID, err := invitor.GetTgId()
	if e.IsNonNil(err) {
		return err
	}

	if eraw = tx.Commit(); eraw != nil {
		return e.FromError(eraw, "failed to commit transaction").WithSeverity(e.Critical)
	}

	return hashe.Emit(constants.OutgoingRoutingKey, messageBuilder.Build(invitorTgID))
}

// Runs all referral functions and sends all needed messages
// Takes: database, invited user, hashe, connection status(true if invited user connected bot)
func referralMain(db *pg.DB, actor *models.Telegramuser, hashe *h.HandlerChainHashe, connected bool) *e.ErrorInfo {
	if !connected {
		if err := sendReferral(db, actor, hashe, false); err != nil {
			return err
		}

		return updateReferral(db, actor, false)
	}

	err := sendReferral(db, actor, hashe, true)
	if e.IsNonNil(err) {
		return err
	}

	return updateReferral(db, actor, true)
}
