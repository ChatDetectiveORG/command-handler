package endpoints

import (
	"time"

	e "github.com/ChatDetectiveORG/shared/errors"
	models "github.com/ChatDetectiveORG/shared/postgresModels"
	utils "github.com/ChatDetectiveORG/shared/utils"
	"github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
	tele "gopkg.in/telebot.v4"
)

// GetUserByTgID fetches a Telegramuser from the database by Telegram user ID.
func GetUserByTgID(db *pg.DB, tgUserID int64) (*models.Telegramuser, *e.ErrorInfo) {
	user := &models.Telegramuser{}
	err := user.GetByTelegramID(db, tgUserID)
	if e.IsNonNil(err) {
		return nil, err
	}
	return user, e.Nil()
}

// GetUserSettings fetches UserSettings for the given user.
func GetUserSettings(db *pg.DB, user *models.Telegramuser) (*models.UserSettings, *e.ErrorInfo) {
	settings := &models.UserSettings{}
	eraw := db.Model(settings).Where("linked_user_id = ?", user.ID).Select()
	if eraw != nil {
		return nil, e.FromError(eraw, "failed to get user settings").WithSeverity(e.Notice)
	}
	return settings, e.Nil()
}

// GetUserByTgIDWithSettings fetches both the user and their settings in two queries.
func GetUserByTgIDWithSettings(db *pg.DB, tgUserID int64) (*models.Telegramuser, *models.UserSettings, *e.ErrorInfo) {
	user, err := GetUserByTgID(db, tgUserID)
	if e.IsNonNil(err) {
		return nil, nil, err
	}
	settings, err := GetUserSettings(db, user)
	if e.IsNonNil(err) {
		return nil, nil, err
	}
	return user, settings, e.Nil()
}

// ShiftEntities returns a copy of the entity slice with all offsets shifted by delta.
func ShiftEntities(entities []tele.MessageEntity, delta int) []tele.MessageEntity {
	shifted := make([]tele.MessageEntity, len(entities))
	for i, ent := range entities {
		shifted[i] = ent
		shifted[i].Offset += delta
	}
	return shifted
}

// ContactsForUser returns all UserRelations entries for the given user.
// Each entry's "other" user (the one that is not the query user) is populated.
func ContactsForUser(db *pg.DB, user *models.Telegramuser) ([]models.UserRelations, *e.ErrorInfo) {
	var relations []models.UserRelations
	eraw := db.Model(&relations).
		WhereGroup(func(q *orm.Query) (*orm.Query, error) {
			return q.WhereOr("first_user_id = ?", user.ID).
				WhereOr("second_user_id = ?", user.ID), nil
		}).
		Relation("FirstUser").
		Relation("SecondUser").
		Select()
	if eraw != nil {
		return nil, e.FromError(eraw, "failed to get user relations").WithSeverity(e.Notice)
	}
	return relations, e.Nil()
}

// OtherUserInRelation returns the user that is NOT the provided user in the relation.
func OtherUserInRelation(relation models.UserRelations, user *models.Telegramuser) *models.Telegramuser {
	if relation.FirstUser != nil && string(relation.FirstUser.ID) == string(user.ID) {
		return relation.SecondUser
	}
	return relation.FirstUser
}

// BuildReferralLink builds the referral start link for the given user.
// The first 10 characters of the user's IDHash are used as the start parameter.
func BuildReferralLink(user *models.Telegramuser) string {
	startParam := user.ReferralCode

	return "https://t.me/MajorFanOfInnokentii_bot?start=" + startParam
}

// AnswerCallbackBanner sends a silent banner callback answer (shows at top, no popup).
func AnswerCallbackBanner(text string, cb *tele.Callback) *tele.CallbackResponse {
	return &tele.CallbackResponse{
		Text:      text,
		ShowAlert: false,
	}
}

// UserRelatedNonBotUsers returns related users who are NOT bot business users
// (i.e., they have an empty BusinessConnectionIDHash).
func UserRelatedNonBotUsers(db *pg.DB, user *models.Telegramuser) ([]*models.Telegramuser, *e.ErrorInfo) {
	relations, err := ContactsForUser(db, user)
	if e.IsNonNil(err) {
		return nil, err
	}

	var result []*models.Telegramuser
	for i := range relations {
		other := OtherUserInRelation(relations[i], user)
		if other != nil && other.BusinessConnectionIDHash == "" {
			result = append(result, other)
		}
	}
	return result, e.Nil()
}

// UpdateBusinessConnectionIDHash sets (or clears) the user's businessConnectionIDHash.
func UpdateBusinessConnectionIDHash(db *pg.DB, user *models.Telegramuser, businessConnectionID string) *e.ErrorInfo {
	if businessConnectionID == "" {
		user.BusinessConnectionIDHash = ""
	} else {
		hash, err := utils.ToSecureHash(businessConnectionID)
		if e.IsNonNil(err) {
			return err
		}
		user.BusinessConnectionIDHash = hash
	}

	user.UpdatedAt = time.Now()
	_, eraw := db.Model(user).WherePK().Column("business_connection_id_hash", "updated_at").Update()
	if eraw != nil {
		return e.FromError(eraw, "failed to update business connection id hash").WithSeverity(e.Notice)
	}
	return e.Nil()
}
