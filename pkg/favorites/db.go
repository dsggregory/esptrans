package favorites

import (
	"fmt"
	"gorm.io/gorm/logger"
	"net/url"
	"os"
	"strings"

	gorm_logrus "github.com/onrik/gorm-logrus"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// New connects to the specified database by |href|
//   - mysql://user:password@host:3306/dbname?charset=utf8&parseTime=True&loc=Local
//   - postgresql://user:password@host:5432/dbname
//   - file://path-to-sqlite3-db-file
//
// For a postgres v15 connection (with 'postgres' user as admin)
//   - CREATE DATABASE EXAMPLE_DB;
//   - CREATE USER EXAMPLE_USER WITH ENCRYPTED PASSWORD 'Sup3rS3cret';
//   - GRANT ALL PRIVILEGES ON DATABASE EXAMPLE_DB TO EXAMPLE_USER;
//   - \c EXAMPLE_DB postgres
//   - # You are now connected to database "EXAMPLE_DB" as user "postgres".
//   - GRANT ALL ON SCHEMA public TO EXAMPLE_USER;
func New(href string, debug bool) (*gorm.DB, error) {
	if href == "" {
		return nil, fmt.Errorf("empty DB DSN")
	}
	u, err := url.Parse(href)
	if err != nil {
		return nil, err
	}

	switch u.Scheme {
	case "postgres", "postgresql":
		dbname := u.Path[1:]
		gcfg := &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
			//Logger: gorm_logrus.New(),
		}
		var db *gorm.DB
		var err error
		for i := 0; i < 2; i++ {
			db, err = gorm.Open(postgres.Open(href), gcfg)
			if err != nil {
				if strings.Contains(err.Error(), "SQLSTATE 3D000") {
					// database does not exist
					u.Path = "/postgres"
					nh := u.String()
					db, err = gorm.Open(postgres.Open(nh), gcfg)
					if err != nil {
						break
					}
					if err := db.Exec(fmt.Sprintf("CREATE DATABASE %s;", dbname)).Error; err != nil {
						break
					}
					continue
				} else {
					break
				}
			}
			break
		}
		return db, err
	case "file":
		return openSqlite3(u.Path)
	default:
		return nil, fmt.Errorf("unknown database DSN scheme \"%s\"", href)
	}
}

func openSqlite3(path string) (*gorm.DB, error) {
	var l logger.Interface
	if s := os.Getenv("DB_LOG_SILENT"); s != "" {
		l = logger.Default.LogMode(logger.Silent)
	} else {
		l = gorm_logrus.New()
		l.LogMode(logger.Info)
	}

	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: l,
	})
	if err != nil {
		return nil, err
	}

	return db, nil
}

// TestDB creates an sqlite test database if dbpath exists as a file. Get a shell to the db with:
//   - sqlite3 ./pkg/api/test_db.db
//
// If it starts with "postgres://", then it returns a connection to a live postgres test database.
func TestDB(dbpath string) *gorm.DB {
	var tdb *gorm.DB

	if dbpath[:8] != "postgres" {
		db, err := openSqlite3(dbpath)
		if err != nil {
			return nil
		}
		tdb = db
	} else {
		testdb, err := New(dbpath, true)
		if err != nil {
			panic(err)
		}
		tdb = testdb
	}
	return tdb
}

// DropTestDB drop the test database
func DropTestDB(dbpath string) error {
	if dbpath[:8] != "postgres" {
		if err := os.Remove(dbpath); err != nil {
			return err
		}
	} else {
		u, err := url.Parse(dbpath)
		if err != nil {
			return err
		}
		dbname := u.Path[1:]
		u.Path = "/postgres"
		nh := u.String()

		tdb, err := New(nh, true)
		if err != nil {
			return err
		}
		// TODO won't delete while another connection has it open
		err = tdb.Exec(fmt.Sprintf("drop database %s", dbname)).Error
		return err
	}
	return nil
}

// AutoMigrate creates and alters the tables as needed between releases.
// Note that it does not drop columns that have been removed from the model.
func AutoMigrate(db *gorm.DB) error {
	err := db.AutoMigrate(
		&Favorite{},
	)
	if err != nil {
		return err
	}

	return nil
}
