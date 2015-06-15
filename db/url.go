package db

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/AndrewVos/ancientcitadel/slug"
	"github.com/dustin/go-humanize"
)

type URL struct {
	ID           int       `db:"id"`
	CreatedAt    time.Time `db:"created_at"`
	Title        string    `db:"title"`
	SourceURL    string    `db:"source_url"`
	URL          string    `db:"url"`
	WEBMURL      string    `db:"webmurl"`
	MP4URL       string    `db:"mp4url"`
	ThumbnailURL string    `db:"thumbnail_url"`
	Width        int       `db:"width"`
	Height       int       `db:"height"`
	NSFW         bool      `db:"nsfw"`
	Views        int       `db:"views"`

	// never used, just here to appease sqlx
	TSV    string  `db:"tsv" json:"-"`
	Query  string  `db:"query" json:"-"`
	Random float64 `db:"random" json:"-"`
}

func (u URL) ToJSON() (string, error) {
	b, err := json.MarshalIndent(u, " ", "")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (u URL) Permalink() string {
	slug := slug.Slug(u.ID, u.Title)
	return fmt.Sprintf("/gif/%v", slug)
}

func (u URL) HumanCreatedAt() string {
	return humanize.Time(u.CreatedAt)
}

func (u URL) ShareMarkdown() string {
	return fmt.Sprintf("![%s](%s)", u.URL, u.URL)
}

func GetRandomURL(nsfw bool) (URL, error) {
	db, err := db()
	if err != nil {
		return URL{}, err
	}
	var url URL
	err = db.Get(&url, "SELECT * FROM urls WHERE nsfw=$1 ORDER BY random() LIMIT 1", nsfw)
	return url, err
}

func GetURLs(query string, nsfw bool, page int, pageSize int) ([]URL, error) {
	db, err := db()
	if err != nil {
		return nil, err
	}

	var urls []URL

	if query != "" {
		wordFinder := regexp.MustCompile("\\w+")
		queryParts := wordFinder.FindAllString(query, -1)
		tSearchQuery := strings.Join(queryParts, "&")

		err = db.Select(&urls, `
	SELECT * FROM urls,
		to_tsquery('pg_catalog.english', $1) AS query
		WHERE nsfw=$2
		AND (tsv @@ query)
		ORDER BY
			ts_rank_cd(tsv, query) DESC,
			id
		LIMIT $3 OFFSET $4`,
			tSearchQuery, nsfw, pageSize, (page-1)*pageSize)
	} else {
		err = db.Select(&urls, `
	SELECT * FROM urls
		WHERE nsfw = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`,
			nsfw, pageSize, (page-1)*pageSize)
	}

	return urls, err
}

func ExistsInDB(url URL) (int, error) {
	db, err := db()
	if err != nil {
		return 0, err
	}

	var ids []int
	err = db.Select(&ids, "SELECT id FROM urls WHERE url = $1 OR source_url = $2 LIMIT 1;", url.URL, url.SourceURL)

	if err != nil {
		return 0, err
	}

	if len(ids) == 1 {
		return ids[0], nil
	}
	return 0, nil
}

func UpdateURL(id int, url URL) error {
	db, err := db()
	if err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE urls SET nsfw = $1 WHERE id = $2`,
		url.NSFW,
		id,
	)
	return err
}

func SaveURL(url *URL) error {
	db, err := db()
	if err != nil {
		return err
	}

	_, err = db.Exec(`
	INSERT INTO urls (
		created_at, title, nsfw, url, source_url, webmurl, mp4url, thumbnail_url, width, height
	) VALUES (
		$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
	)`,
		url.CreatedAt,
		url.Title,
		url.NSFW,
		url.URL,
		url.SourceURL,
		url.WEBMURL,
		url.MP4URL,
		url.ThumbnailURL,
		url.Width,
		url.Height,
	)
	if err != nil {
		return err
	}
	return nil
}

func GetTopURLs(nsfw bool, page int, pageSize int) ([]URL, error) {
	db, err := db()
	if err != nil {
		return nil, err
	}

	var urls []URL
	err = db.Select(&urls, `
		SELECT urls.*,
			COUNT(url_views.created_at) AS views
			FROM urls
			INNER JOIN url_views on url_views.url_id = urls.id
			WHERE nsfw = $1
			GROUP BY urls.id
			ORDER BY views DESC
			LIMIT $2 OFFSET $3
		`,
		nsfw, pageSize, (page-1)*pageSize)

	return urls, err
}

func GetURL(id int) (*URL, error) {
	db, err := db()
	if err != nil {
		return nil, err
	}
	var urls []URL
	err = db.Select(&urls, `SELECT * FROM urls WHERE id = $1 LIMIT 1`, id)
	if err != nil {
		return nil, err
	}
	if len(urls) > 0 {
		return &urls[0], nil
	}
	return nil, err
}

func GetURLCount() (int, error) {
	db, err := db()
	if err != nil {
		return 0, err
	}
	var count int
	err = db.Get(&count, "SELECT COUNT(*) FROM urls")
	return count, err
}

func GetShuffledURLs(nsfw bool, page int, pageSize int) ([]URL, error) {
	db, err := db()
	if err != nil {
		return nil, err
	}

	var urls []URL
	err = db.Select(&urls, `
		SELECT * FROM urls
			WHERE nsfw = $1
			ORDER BY random(), id
			LIMIT $2 OFFSET $3`,
		nsfw, pageSize, (page-1)*pageSize)

	return urls, err
}

func StoreURLView(url URL) error {
	db, err := db()
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO url_views (url_id) VALUES ($1)`, url.ID)
	return err
}
