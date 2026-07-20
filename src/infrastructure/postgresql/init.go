package postgresql

import (
	"os"
	"sync"

	e "github.com/ChatDetectiveORG/shared/errors"

	// requiredModels "github.com/ChatDetectiveORG/command-handler/src/infrastructure/postgresql/requiredModels"

	"github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"

	models "github.com/ChatDetectiveORG/shared/postgresModels"
)

var (
	db   *pg.DB
	once sync.Once
)

func GetDB() *pg.DB {
	once.Do(func() {
		db = pg.Connect(&pg.Options{
			Addr:     os.Getenv("DB_HOST") + ":" + os.Getenv("DB_PORT"),
			User:     os.Getenv("POSTGRES_USER"),
			Password: os.Getenv("POSTGRES_PASSWORD"),
			Database: os.Getenv("POSTGRES_DB"),
			PoolSize: 20, // Устанавливаем разумный размер пула
		})
	})
	return db
}

func InitPostgresql() *e.ErrorInfo {
	db := GetDB()

	models := []interface{}{
		(*models.Telegramuser)(nil),
		(*models.UserSettings)(nil),
		(*models.UserRelations)(nil),
		(*models.UserLevels)(nil),
		(*models.Admin)(nil),
		(*models.Payment)(nil),
		(*models.Mirror)(nil),
		(*models.MirrorFile)(nil),
		(*models.Message)(nil),
		(*models.MessageVersion)(nil),
		(*models.Referral)(nil),
		(*models.UserConsent)(nil),
	}

	for _, model := range models {
		err := db.Model(model).CreateTable(&orm.CreateTableOptions{
			IfNotExists: true,
			Temp:        false,
		})
		if err != nil {
			return e.FromError(err, "error creating table")
		}
	}

	err := CreateIndexes(db)
	if e.IsNonNil(err) {
		return e.Wrap(err).PushStack()
	}

	return nil
}

func CreateIndexes(db *pg.DB) error {
	// Индекс для поиска, где пользователь — первый в связке
	_, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_relations_first_user ON user_relations (first_user_id)`)
	if err != nil {
		return err
	}

	// Индекс для поиска, где пользователь — второй в связке
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_relations_second_user ON user_relations (second_user_id)`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_messages_sender_id_hash ON messages (sender_id_hash)`)

	return err
}
