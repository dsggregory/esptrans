package favorites

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"strings"
)

type DbDialect int

const (
	DialectSqlite = iota
	DialectPostgres
)

type DBService struct {
	Dialect DbDialect
	db      *gorm.DB
}

func (s *DBService) Db() *gorm.DB {
	return s.db
}

func (s *DBService) SetDb(db *gorm.DB) {
	s.db = db
}

func NewDBService(dsn string) (*DBService, error) {
	dbg := logrus.GetLevel() == logrus.DebugLevel
	db, err := New(dsn, dbg)
	if err != nil {
		return nil, err
	}
	if err = AutoMigrate(db); err != nil {
		return nil, fmt.Errorf("%w; cannot auto migrate DB", err)
	}

	var dialect DbDialect = DialectPostgres
	if strings.Contains(dsn, "sqlite") {
		dialect = DialectSqlite
	}
	return &DBService{db: db, Dialect: dialect}, nil
}

func (s *DBService) AddFavorite(fav *Favorite) (*Favorite, error) {
	tx := s.db.Begin()
	if err := tx.Create(&fav).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if fav.ID == 0 {
		if err := tx.Where(fav.ID).Find(&fav).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Where("id=?", fav.ID).Find(fav).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	return fav, tx.Commit().Error
}