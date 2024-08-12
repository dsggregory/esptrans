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
func (s *DBService) SelectRandomFavorite() (*Favorite, error) {
	// SELECT * FROM table WHERE id IN (SELECT id FROM table ORDER BY RANDOM() LIMIT x)
	var fav Favorite

	err := s.db.Raw("SELECT * FROM favorites WHERE id IN (SELECT id FROM favorites WHERE deleted_at is null ORDER BY RANDOM() LIMIT 1)").Find(&fav).Error
	return &fav, err
}

// SelectFavorite select a specific favorite
func (s *DBService) SelectFavorite(id uint) (*Favorite, error) {
	var fav Favorite

	err := s.db.Where("id=?", id).First(&fav).Error
	return &fav, err
}

// SelectFavoriteSource select a specific favorite by source column
func (s *DBService) SelectFavoriteSource(source string) (*Favorite, error) {
	var fav Favorite

	err := s.db.Where("source=?", source).First(&fav).Error
	return &fav, err
}

// DeleteFavorite delete a specific favorite
func (s *DBService) DeleteFavorite(id uint) error {
	var fav Favorite

	fav.ID = uint(id)
	err := s.db.Delete(&fav).Error
	return err
}

func (s *DBService) UpdateFavorite(fav *Favorite) error {
	if fav.ID == 0 {
		return gorm.ErrRecordNotFound
	}

	havc, err := s.SelectFavorite(fav.ID)
	if havc == nil {
		if err == nil {
			return gorm.ErrRecordNotFound
		} else {
			return err
		}
	}

	tx := s.db.Begin()
	txwhere := tx.Where("id=?", fav.ID)
	if err := txwhere.Save(fav).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := txwhere.Find(fav).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error

}
