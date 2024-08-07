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

// SelectRandomFavorites for flash cards
func (s *DBService) SelectRandomFavorites(limit int) ([]Favorite, error) {
	// SELECT * FROM table WHERE id IN (SELECT id FROM table ORDER BY RANDOM() LIMIT x)
	var favs []Favorite

	err := s.db.Raw("SELECT * FROM favorites WHERE id IN (SELECT id FROM favorites ORDER BY RANDOM() LIMIT ?)", limit).Find(&favs).Error
	return favs, err
}

// SelectRandomFavorite select ONE favorite for flash cards
func (s *DBService) SelectRandomFavorite() (Favorite, error) {
	// SELECT * FROM table WHERE id IN (SELECT id FROM table ORDER BY RANDOM() LIMIT x)
	var fav Favorite

	err := s.db.Raw("SELECT * FROM favorites WHERE id IN (SELECT id FROM favorites WHERE deleted_at is null ORDER BY RANDOM() LIMIT 1)").Find(&fav).Error
	return fav, err
}

// SelectFavorite select a specific favorite
func (s *DBService) SelectFavorite(id int) (Favorite, error) {
	var fav Favorite

	err := s.db.Where("id=?", id).Find(&fav).Error
	return fav, err
}

// DeleteFavorite delete a specific favorite
func (s *DBService) DeleteFavorite(id int) error {
	var fav Favorite

	fav.ID = uint(id)
	err := s.db.Delete(&fav).Error
	return err
}
