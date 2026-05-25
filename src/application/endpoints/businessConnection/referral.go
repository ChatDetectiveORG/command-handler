package businessconnection

import (
	"errors"
	"math"
	"slices"
	"strconv"
	"time"

	shared "github.com/ChatDetectiveORG/command-handler/src/application/endpoints"
	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	models "github.com/ChatDetectiveORG/shared/postgresModels"
	"github.com/ChatDetectiveORG/shared/telegram"
	"github.com/go-pg/pg/v10"
	tele "gopkg.in/telebot.v4"
)

// Gets used link owner and referral relation between users
// Takse: transaction, actor (invited user)
// Returns: referral, invitor (link owner), error
func getReferralAndLinkOwner(tx *pg.Tx, actor *models.Telegramuser) (*models.Referral, *models.Telegramuser, *e.ErrorInfo) {
	referral := &models.Referral{}
	err := e.Wrap(tx.Model(referral).Where("invited_user_id = ?", actor.ID).Select())
	if e.IsNonNil(err) {
		if err.Err.Error() != "no rows in result set" {
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
		referral.FixedMoneyReward = shared.ReferralBonusRub
	case models.ReferralBonusLevels:
		referral.ActualUntil = time.Now().Add(time.Duration(shared.ReferralLevelsDurationSec) * time.Second)
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

// Takes non-considered yet referral relations and adds new bonus levels accoardingly to the current threshold
// Old levels stay untouched even if threshold risen
// Takes: transaction, untracked relations (referrals that were not considered yet), level recipient user ID
// Returns: number of levels added, error
func recountLevels(tx *pg.Tx, untrackedRalations []models.Referral, levelRecipientUserID []byte) (int, *e.ErrorInfo) {
	var levelsAdded int
	threshold := shared.ReferralBonusThresholdLevels
	now := time.Now()
	defaultBonusEnd := now.Add(time.Duration(shared.ReferralLevelsDurationSec) * time.Second).Unix()

	for i := 0; i+threshold <= len(untrackedRalations); i += threshold {
		addedRelationsDurations := make([]int64, 0, threshold)
		addedRelationsIDs := make([]int, 0, threshold)

		for j := i; j < i+threshold; j++ {
			ref := untrackedRalations[j]
			u := ref.ActualUntil.Unix()
			if ref.ActualUntil.IsZero() || u <= 0 {
				u = defaultBonusEnd
			}
			addedRelationsDurations = append(addedRelationsDurations, u)
			addedRelationsIDs = append(addedRelationsIDs, ref.ID)
		}

		newLevel := models.UserLevels{
			LinkedUserID:      levelRecipientUserID,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
			Level:             shared.ReferralBonusLevelsPerUnlock,
			UntilTimestamp:    slices.Min(addedRelationsDurations),
			IsReferralBonus:   true,
			LinkedReferralIDs: addedRelationsIDs,
		}

		_, eRaw := tx.Model(&newLevel).Insert()
		if e.IsNonNil(eRaw) {
			return levelsAdded, e.Wrap(eRaw)
		}

		levelsAdded += 1
	}

	return levelsAdded, e.Nil()
}

// Gets untracked relations (referrals that were not considered yet)
// Takes: transaction, invitor user ID, referral
// Returns: untracked relations, error
func getUntrackedRelations(tx *pg.Tx, invitorID []byte, referral *models.Referral) ([]models.Referral, *e.ErrorInfo) {
	var untrackedRalations []models.Referral
	err := e.Wrap(tx.Model(&untrackedRalations).
		Where("invitor_id = ?", invitorID).
		Where("id NOT IN (SELECT unnest(linked_referral_ids) FROM user_levels WHERE linked_user_id = ?)", invitorID).
		Order("actual_until ASC").
		Select(),
	)
	if e.IsNonNil(err) {
		return nil, err
	}

	return untrackedRalations, e.Nil()
}

// Handles levels bonus for invitor (adds or removes levels based on connected status)
// Takes: transaction, referral, message builder, connected (true if invited user connected bot)
// Returns: error
func handleLevels(tx *pg.Tx, referral *models.Referral, messageBuilder *telegram.MessageBuilder, connected bool) *e.ErrorInfo {
	invitorID := referral.InvitorID

	err := e.Wrap(tx.Model(&models.UserLevels{}).
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

	untrackedRalations, err := getUntrackedRelations(tx, invitorID, referral)
	if e.IsNonNil(err) {
		return err
	}

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
	}

	levelsAdded, err := recountLevels(tx, untrackedRalations, invitorID)
	if e.IsNonNil(err) {
		return err
	}

	if levelsAdded == 0 {
		messageBuilder.WriteString(
			"🚫",
			telegram.TextFormat{Type: telegram.TextFormatTypeLink}.WithCustomEmojiID("5462882007451185227"),
		).WriteString("Твой уровень снижен на 1.\n").WriteString(
			"Сейчас он составляет: " + strconv.Itoa(levelSummary.Level),
		)

		return e.Nil()
	}

	messageBuilder.WriteString(
		"Твой уровень сейчас составляет " + strconv.Itoa(levelSummary.Level+levelsAdded) + "\n\n",
	).WriteString(
		"Ты получил " + strconv.Itoa(levelsAdded) + " уровней\nПриведи " + 
		strconv.Itoa(shared.ReferralBonusThresholdLevels-(len(untrackedRalations)-levelsAdded*shared.ReferralBonusThresholdLevels)) + 
		" пользователей чтобы получить ещё один бонусный уровень!",

	)

	return e.Nil()
}

// Writes removed or added money bonus text
// Takes: message builder, connected (true if invited user connected bot)
func handleMoney(messageBuilder *telegram.MessageBuilder, connected bool) {
	if connected {
		messageBuilder.WriteString(
			"🔥", telegram.TextFormat{Type: telegram.TextFormatTypeLink}.WithCustomEmojiID("5256047523620995497"),
		).WriteString(
			"Если он не будет отключать бота до ",
		).WriteString(
			time.Now().Add(time.Duration(shared.ReferralDiscountDurationSec)*time.Second + time.Hour*24).Format("02.01.2006"),
		).WriteString(" ты получишь " + strconv.FormatInt(shared.ReferralBonusRub, 10) + " рублей на внутренний счёт бота")

		return
	}

	messageBuilder.WriteString(
		"🚫", telegram.TextFormat{Type: telegram.TextFormatTypeLink}.WithCustomEmojiID("5462882007451185227"),
	).WriteString("Ты не получишь выплат за него\n").WriteString(
		"Чтобы получить денги за него, он должен подключить бота и пользоваться им без перерыва в течение ",
	).WriteString(
		strconv.FormatFloat(math.Ceil(float64(time.Duration(shared.ReferralDiscountDurationSec).Hours()/24)), 'f', -1, 64) + " дней",
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

	messageBuilder := telegram.MessageBuilder{Mdv2Enabled: true}
	messageBuilder.WriteString("🔝", telegram.TextFormat{Type: telegram.TextFormatTypeLink}.WithCustomEmojiID("5463071033256848094"))

	actorFullName, err := actor.GetFullName()
	if e.IsNonNil(err) {
		return err
	}
	actorTgID, err := actor.GetTgId()
	if e.IsNonNil(err) {
		return err
	}
	messageBuilder.WriteString("Пользователь ")
	messageBuilder.WriteString(actorFullName, telegram.TextFormat{Type: telegram.TextFormatTypeLink}.WithUserMention(actorTgID))

	if connected {
		messageBuilder.WriteString(" подключил бота!\n\n")
	} else {
		messageBuilder.WriteString(" отключил бота!\n\n")
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

	return hashe.WithParseMode(true).Emit(shared.OutgoingRoutingKey, messageBuilder.Build(invitorTgID))
}

// Runs all referral functions and sends all needed messages
// Takes: database, invited user, hashe, connection status(true if invited user connected bot)
func referralMain(db *pg.DB, actor *models.Telegramuser, hashe *h.HandlerChainHashe, connected bool) *e.ErrorInfo {
	if !connected {
		if err := updateReferral(db, actor, false); e.IsNonNil(err) {
			return err
		}
		return sendReferral(db, actor, hashe, false)
	}

	err := sendReferral(db, actor, hashe, true)
	if e.IsNonNil(err) {
		return err
	}

	return updateReferral(db, actor, true)
}