package businessconnection

import (
	"log"

	"github.com/ChatDetectiveORG/command-handler/src/infrastructure/postgresql"
	e "github.com/ChatDetectiveORG/shared/errors"
	. "github.com/ChatDetectiveORG/shared/messageBuilder"
	postgresmodels "github.com/ChatDetectiveORG/shared/postgresModels"
	"github.com/go-pg/pg/v10"
	tele "gopkg.in/telebot.v4"
)

// Replace old business connection id hash to new in Messages table
func ReplaceOldBusinessConnectionIdHash(db *pg.DB, userID int64) *e.ErrorInfo {
	var user = &postgresmodels.Telegramuser{}
	err := user.GetByTelegramID(db, userID)
	if e.IsNonNil(err) {
		return err
	}

	updatedFields := &postgresmodels.Message{
		BusinessConnectionIDHash: user.BusinessConnectionIDHash,
	}
	_, eRaw := db.Model(updatedFields).
    Column("business_connection_id_hash").
    Where("business_connection_id_hash = ?", user.LastBusinessConnectionIDHash).
    Update()

	if e.IsNonNil(eRaw) {
		return e.FromError(eRaw, "failed to update business connection id hash").WithSeverity(e.Notice)
	}

	return e.Nil()
}

func buildConnectedMessage(chatID int64) *tele.Message {
	db := postgresql.GetDB()
	err := ReplaceOldBusinessConnectionIdHash(db, chatID)
	if e.IsNonNil(err) {
		log.Println("failed to replace old business connection id hash", err)
	}

	messageBuilder := MessageBuilder{Mdv2Enabled: true}

	messageBuilder.Write(
		B(T("Бот подключен, все работает как надо!", Args{NoNewline: true})), E("5463423955014529788", "👌"),
		T("\n"),
		T("nТеперь:"),
		E("5465465194056525619"), T("Ты будешь получать уведомления, если кто-то удалит или изменит сообщения в личных чатах"),
		E("5465465194056525619"), T("Ты сможешь скачивать фото, видео, голосовые сообщения и кружочки которые обычно исчезают после одного просмотра"),
		E("5465465194056525619"), T("У тебя будет возможность восстановить чат даже после его удаления"),
		T(""),
		T("В общем, полный контроль над собеседником!"),
	)

	return messageBuilder.Build(chatID)
}
