package db

import (
	"time"

	_ "github.com/lib/pq"
)

type DownloadResult struct {
	CreatedAt time.Time `db:"created_at"`
	URL       string    `db:"url"`
	Success   bool      `db:"success"`
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
