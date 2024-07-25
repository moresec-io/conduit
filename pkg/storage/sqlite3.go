package storage

import (
	"net/url"
	"path/filepath"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func NewSqlite3(dataDir string, dbName, option string, debug bool) (*gorm.DB, error) {
	if option == "" {
		option = "parseTime=true"
	}
	dsn := filepath.Join(dataDir, dbName)
	dsn += "?" + option
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, err
	}

	values := u.Query()
	values.Set("cache", "shared")
	values.Set("mode", "rwc")
	values.Set("_journal_mode", "WAL")
	u.RawQuery = values.Encode()

	db, err := gorm.Open(
		sqlite.Open(dsn),
		&gorm.Config{
			SkipDefaultTransaction:                   true,
			DisableNestedTransaction:                 true,
			PrepareStmt:                              true,
			AllowGlobalUpdate:                        true,
			DisableForeignKeyConstraintWhenMigrating: true,
			CreateBatchSize:                          100,
		})
	if err != nil {
		return nil, err
	}
	if debug {
		db = db.Debug()
	}
	return db, nil
}
