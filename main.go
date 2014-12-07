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
	TotalURLs    int
}

type URL struct {
	CreatedAt time.Time `db:"created_at"`
	Work      string    `db:"work"`
	Title     string    `db:"title"`
	SourceURL string    `db:"source_url"`
	URL       string    `db:"url"`
	WebMURL   string    `db:"webmurl"`
	MP4URL    string    `db:"mp4url"`
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

	totalURLs, err := getURLCount()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println(err)
		return
	}
	page := Page{CurrentPage: 1, TotalURLs: totalURLs}
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

func updateSubReddit(work string, name string) error {
	log.Printf("Downloading %q %q...\n", work, name)
	subReddit := reddit.SubReddit{Name: name}

	for {
		redditURLs, err := subReddit.NextPage()
		if err != nil {
			return err
		}
		if len(redditURLs) == 0 {
			return nil
		}
		log.Printf("Downloaded %d urls from /r/%v\n", len(redditURLs), name)
		for _, redditURL := range redditURLs {
			sourceURL := "https://reddit.com" + redditURL.Permalink

			exists, err := existsInDB(sourceURL)
			if err != nil {
				log.Println(err)
				continue
			}
			if exists {
				log.Printf("%v already stored\n", sourceURL)
				continue
			}

			information, err := gfycat.Gif(redditURL.URL)
			if err != nil {
				log.Println(err)
				continue
			}

			url := URL{
				Work:      work,
				Title:     redditURL.Title,
				SourceURL: sourceURL,
				URL:       redditURL.URL,
				WebMURL:   information.WebMURL,
				MP4URL:    information.MP4URL,
				Width:     information.Width,
				Height:    information.Height,
				CreatedAt: time.Unix(int64(redditURL.Created), 0),
			}
			err = saveURL(url)
			if err != nil {
				log.Println(err)
			}
		}
		time.Sleep(2 * time.Second)
	}
}

func updateRedditForever() {
	sfw := []string{
		"gifs", "perfectloops", "creepy_gif", "noisygifs", "analogygifs",
		"reversegif", "funny_gifs", "funnygifs", "aww_gifs", "wheredidthesodago",
		"AnimalsBeingJerks", "AnimalGIFs", "birdreactiongifs", "CatGifs", "catreactiongifs",
		"Puggifs", "KimJongUnGifs", "SpaceGifs", "physicsgifs", "educationalgifs",
		"chemicalreactiongifs", "mechanical_gifs",
	}
	nsfw := []string{
		"gifsgonewild", "porn_gifs", "PornGifs", "NSFW_SEXY_GIF", "nsfwcelebgifs",
		"adultgifs", "NSFW_GIF", "nsfw_gifs", "porngif", "cutegirlgifs", "Hot_Women_Gifs",
		"randomsexygifs", "TittyDrop", "boobbounce", "boobgifs", "celebgifs",
	}

	go func() {
		for {
			for _, s := range sfw {
				err := updateSubReddit("sfw", s)
				if err != nil {
					log.Println(err)
				}
			}
			for _, s := range nsfw {
				err := updateSubReddit("nsfw", s)
				if err != nil {
					log.Println(err)
				}
			}
		}
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

func existsInDB(sourceURL string) (bool, error) {
	db, err := db()
	if err != nil {
		return false, err
	}

	var count int
	err = db.Get(&count, "SELECT count(*) FROM urls WHERE source_url= $1;", sourceURL)
	if err != nil {
		return false, err
	}
	return count != 0, nil
}

func getURLCount() (int, error) {
	db, err := db()
	if err != nil {
		return 0, err
	}
	var count int
	err = db.Get(&count, "SELECT count(*) FROM urls;")
	if err != nil {
		return 0, err
	}
	return count, nil

}

func saveURL(url URL) error {
	db, err := db()
	if err != nil {
		return err
	}

	_, err = db.Exec(`
	INSERT INTO urls (
		created_at, work, title, url, source_url, webmurl, mp4url, width, height
	) VALUES (
		$1, $2, $3, $4, $5, $6, $7, $8, $9
	)`,
		url.CreatedAt,
		url.Work,
		url.Title,
		url.URL,
		url.SourceURL,
		url.WebMURL,
		url.MP4URL,
		url.Width,
		url.Height,
	)
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
	serveAsset(r, "/assets/scripts/packery.pkgd.min.js")
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
					WHEN duplicate_column THEN RAISE NOTICE 'column source_url already exists in urls.';
				END;
			END;
		$$;

		DO $$
			BEGIN
				BEGIN
					ALTER TABLE urls ADD COLUMN mp4url TEXT;
				EXCEPTION
					WHEN duplicate_column THEN RAISE NOTICE 'column mp4url already exists in urls.';
				END;
			END;
		$$
	`
	db.MustExec(schema)
}
