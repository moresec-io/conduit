package repo

import (
	"github.com/moresec-io/conduit/pkg/manager/config"
	"github.com/moresec-io/conduit/pkg/storage"
	"gorm.io/gorm"
)

type DBDriver string

const (
	DBDriverUnknown = "unknown"
	DBDriverMySQL   = "mysql"
	DBDriverSqlite  = "sqlite"
)

type dao struct {
	db   *gorm.DB
	conf *config.DB
}

func NewDao(conf *config.Config) (*dao, error) {
	dbconf := &conf.DB
	var (
		db  *gorm.DB
		err error
	)
	switch dbconf.Driver {
	case DBDriverMySQL:
		db, err = storage.NewMySQL(dbconf)
		if err != nil {
			return nil, err
		}
		if err = setMaxConn(db, dbconf.MaxOpenConn, dbconf.MaxIdleConn); err != nil {
			return nil, err
		}

	case DBDriverSqlite:
		db, err = storage.NewSqlite3(dbconf.Address, dbconf.DB, dbconf.Options, dbconf.Debug)
		if err != nil {
			return nil, err
		}
		if err = setMaxConn(db, 1, 0); err != nil {
			return nil, err
		}
	}
	if err = db.AutoMigrate(&Cert{}, &CA{}); err != nil {
		return nil, err
	}
	return &dao{db: db, conf: dbconf}, nil
}

func setMaxConn(db *gorm.DB, maxOpenConn int64, maxIdleConn int64) error {
	// 设置链接数限制
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	if maxOpenConn != 0 {
		sqlDB.SetMaxOpenConns(int(maxOpenConn))
	}
	if maxIdleConn != 0 {
		sqlDB.SetMaxIdleConns(int(maxIdleConn))
	}
	return nil
}

func (dao *dao) Close() error {
	var retErr error
	sqlDB, err := dao.db.DB()
	if err != nil {
		retErr = err
	}
	err = sqlDB.Close()
	if err != nil {
		retErr = err
	}
	return retErr
}
