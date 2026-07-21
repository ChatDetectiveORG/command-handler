package exportchat

import (
	"log"

	e "github.com/ChatDetectiveORG/shared/errors"
	models "github.com/ChatDetectiveORG/shared/postgresModels"
	"github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
)

func chatMessageCount(db *pg.DB, user, interlocutor *models.Telegramuser) (int, *e.ErrorInfo) {
	count, eRaw := db.Model((*models.Message)(nil)).
		Where("chat_id_hash = ?", interlocutor.IDHash).
		Where("business_connection_id_hash = ?", user.BusinessConnectionIDHash).
		Where("is_deleted = false").
		Count()
	if e.IsNonNil(eRaw) {
		return 0, e.FromError(eRaw, "failed to count chat messages").WithSeverity(e.Notice)
	}
	return count, e.Nil()
}

// This function check if the user is in contact with those, whos chat are requested
//
// It made to avoid callback-data spoofing, which could lead to a potential security issue.
func checkCallbackPermission(sender *models.Telegramuser, interlocutor *models.Telegramuser, db orm.DB) *e.ErrorInfo {
	eRaw := db.Model(&models.UserRelations{}).
		WhereGroup(func(q *pg.Query) (*pg.Query, error) {
			return q.Where("first_user_id_hash = ? AND second_user_id_hash = ?", sender.IDHash, interlocutor.IDHash), nil
		}).
		WhereOrGroup(func(q *pg.Query) (*pg.Query, error) {
			return q.Where("first_user_id_hash = ? AND second_user_id_hash = ?", interlocutor.IDHash, sender.IDHash), nil
		}).
		Limit(1).
		Select()
	if e.IsNonNil(eRaw) {
		log.Println(eRaw)
		return e.FromError(eRaw, "user is not permited to access this page").WithSeverity(e.Notice)
	}

	return e.Nil()
}
