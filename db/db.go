package db

import (
	"os"
	"time"

	"github.com/AndrewVos/mig"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var database *sqlx.DB

type DownloadResult struct {
	CreatedAt time.Time `db:"created_at"`
	URL       string    `db:"url"`
	Success   bool      `db:"success"`
}

func Migrate() error {
	return mig.Migrate("postgres", databaseURL(), "./migrations")
}

func databaseURL() string {
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		return databaseURL
	} else {
		return "host=/var/run/postgresql dbname=ancientcitadel sslmode=disable"
	}
}

func db() (*sqlx.DB, error) {
	if database == nil {
		db, err := sqlx.Connect("postgres", databaseURL())
		database = db
		return database, err
	} else {
		return database, database.Ping()
	}
}

func StoreDownloadResult(url string, success bool) error {
	db, err := db()
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO download_results (url, success) VALUES ($1, $2)`, url, success)
	return err
}

func GetDownloadResult(url string) (*DownloadResult, error) {
	db, err := db()
	if err != nil {
		return nil, err
	}
	var results []DownloadResult
	err = db.Select(&results, `SELECT * FROM download_results WHERE url = $1 LIMIT 1`, url)
	if len(results) == 1 {
		return &results[0], nil
	}
	return nil, err
}
