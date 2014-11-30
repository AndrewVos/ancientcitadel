package main

import (
	"flag"
	"fmt"
	"github.com/AndrewVos/ancientcitadel/gfycat"
	"github.com/AndrewVos/ancientcitadel/reddit"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"log"
	"net/http"
	"os"
	"path"
	"runtime"
	"strconv"
	"text/template"
	"time"
)

type Page struct {
	URLs         []URL
	CurrentPage  int
	NextPagePath string
}

type URL struct {
	CreatedAt time.Time `db:"created_at"`
	Work      string    `db:"work"`
	Title     string    `db:"title"`
	SourceURL string    `db:"source_url"`
	URL       string    `db:"url"`
	WebMURL   string    `db:"webmurl"`
	Width     int       `db:"width"`
	Height    int       `db:"height"`
}

func handler(w http.ResponseWriter, r *http.Request) {
	template, err := template.ParseFiles("index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Print(err)
		return
	}
	work := "sfw"
	if mux.Vars(r)["work"] == "nsfw" {
		work = "nsfw"
	}
	page := Page{CurrentPage: 1}
	if p := mux.Vars(r)["page"]; p != "" {
		page.CurrentPage, _ = strconv.Atoi(p)
	}

	page.URLs, err = getURLs(work, page.CurrentPage, 20)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Print(err)
		return
	}
	page.NextPagePath = fmt.Sprintf("/%v/%d", work, page.CurrentPage+1)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Print(err)
		return
	}
	err = template.Execute(w, page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Print(err)
		return
	}
}

func serveAsset(r *mux.Router, assetPath string) {
	r.HandleFunc(assetPath, func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, path.Join(".", assetPath))
	})
}

func updateRedditForever() {
	updateReddit := func() {
		redditURLs := reddit.GetRedditURLs()
		urls := []URL{}

		for _, redditURL := range redditURLs {
			information, err := gfycat.Gif(redditURL.URL)
			if err != nil {
				log.Println(err)
				continue
			}
			urls = append(urls, URL{
				Work:      redditURL.Work,
				Title:     redditURL.Title,
				SourceURL: "https://reddit.com" + redditURL.Permalink,
				URL:       redditURL.URL,
				WebMURL:   information.WebMURL,
				Width:     information.Width,
				Height:    information.Height,
				CreatedAt: time.Now(),
			})
		}
		log.Printf("Storing %d urls\n", len(urls))
		err := saveURLs(urls)
		if err != nil {
			log.Println(err)
		}
	}

	go func() {
		updateReddit()
		ticker := time.NewTicker(600 * time.Second)
		go func() {
			for _ = range ticker.C {
				updateReddit()
			}
		}()
	}()
}

func getURLs(work string, page int, pageSize int) ([]URL, error) {
	db, err := db()
	if err != nil {
		return nil, err
	}

	rows, err := db.Queryx(`
	SELECT * FROM urls
		WHERE work = $1
		ORDER BY created_at DESC
		LIMIT $2
		OFFSET $3`,
		work, pageSize, page*pageSize)
	if err != nil {
		return nil, err
	}
	var urls []URL
	for rows.Next() {
		var url URL
		err := rows.StructScan(&url)
		if err != nil {
			return nil, err
		}
		urls = append(urls, url)
	}
	return urls, nil
}

func saveURLs(urls []URL) error {
	db, err := db()
	if err != nil {
		return err
	}
	tx, err := db.Beginx()
	if err != nil {
		return err
	}

	_, err = tx.Exec("DELETE FROM urls")
	if err != nil {
		tx.Rollback()
		return err
	}

	for _, url := range urls {
		_, err := tx.Exec(`
	INSERT INTO urls (
		created_at, work, title, url, source_url, webmurl, width, height
	) VALUES (
		$1, $2, $3, $4, $5, $6, $7, $8
	)`,
			url.CreatedAt,
			url.Work,
			url.Title,
			url.URL,
			url.SourceURL,
			url.WebMURL,
			url.Width,
			url.Height,
		)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}

func main() {
	runtime.GOMAXPROCS(4)
	migrate()

	updateRedditForever()

	port := flag.String("port", "8080", "the port to bind to")
	flag.Parse()

	r := mux.NewRouter()
	serveAsset(r, "/assets/styles/main.css")
	serveAsset(r, "/assets/styles/items.css")
	serveAsset(r, "/assets/scripts/gifs.js")
	serveAsset(r, "/assets/scripts/instantclick.min.js")
	serveAsset(r, "/assets/images/loading.gif")
	r.Handle("/", handlers.LoggingHandler(os.Stdout, http.HandlerFunc(handler)))
	r.Handle("/{work}/{page}", handlers.LoggingHandler(os.Stdout, http.HandlerFunc(handler)))

	http.Handle("/", r)
	fmt.Printf("Starting on port %v\n", *port)
	err := http.ListenAndServe(":"+*port, nil)
	log.Fatal(err)
}

var database *sqlx.DB

func db() (*sqlx.DB, error) {
	if database == nil {
		if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
			db, err := sqlx.Connect("postgres", databaseURL)
			database = db
			return db, err
		}
		db, err := sqlx.Connect("postgres", "host=/var/run/postgresql dbname=ancientcitadel sslmode=disable")
		database = db
		return database, err
	} else {
		return database, database.Ping()
	}
}

func migrate() {
	db, err := db()
	if err != nil {
		log.Fatalln(err)
	}

	schema := `
		CREATE TABLE IF NOT EXISTS urls(
			created_at TIMESTAMP,
			work    TEXT,
			title   TEXT,
			url     TEXT,
			webmurl TEXT,
			width   INTEGER,
			height  INTEGER
		);
		DO $$
			BEGIN
				BEGIN
					ALTER TABLE urls ADD COLUMN source_url TEXT;
				EXCEPTION
					WHEN duplicate_column THEN RAISE NOTICE 'column source_url already exists in url.';
				END;
			END;
		$$
	`
	db.MustExec(schema)
}
