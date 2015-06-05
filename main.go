package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path"
	"runtime"
	"strconv"
	"text/template"
	"time"

	"github.com/AndrewVos/ancientcitadel/gfycat"
	"github.com/AndrewVos/ancientcitadel/reddit"
	"github.com/AndrewVos/mig"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type Page struct {
	URLs        []URL
	CurrentPage int
	NextPage    *url.URL
}

type URL struct {
	CreatedAt time.Time `db:"created_at"`
	Title     string    `db:"title"`
	SourceURL string    `db:"source_url"`
	URL       string    `db:"url"`
	WebMURL   string    `db:"webmurl"`
	MP4URL    string    `db:"mp4url"`
	Width     int       `db:"width"`
	Height    int       `db:"height"`
	NSFW      bool      `db:"nsfw"`

	// never used, just here to appease sqlx
	TSV   string `db:"tsv"`
	Query string `db:"query"`
}

func (u *URL) ToJSON() (string, error) {
	b, err := json.MarshalIndent(u, " ", "")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func handler(w http.ResponseWriter, r *http.Request) {
	template, err := template.ParseFiles("index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Print(err)
		return
	}
	nsfw := mux.Vars(r)["work"] == "nsfw"

	page := Page{CurrentPage: 1}
	if p := r.URL.Query().Get("page"); p != "" {
		page.CurrentPage, _ = strconv.Atoi(p)
	}

	query := r.URL.Query().Get("q")
	page.URLs, err = getURLs(query, nsfw, page.CurrentPage, 20)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Print(err)
		return
	}

	work := "sfw"
	if nsfw {
		work = "nsfw"
	}

	page.NextPage, _ = url.Parse(fmt.Sprintf("/%v", work))
	q := page.NextPage.Query()
	if query != "" {
		q.Set("q", query)
	}
	q.Set("page", fmt.Sprintf("%v", page.CurrentPage+1))
	page.NextPage.RawQuery = q.Encode()

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

func apiRandomHandler(w http.ResponseWriter, r *http.Request) {
	nsfw := mux.Vars(r)["work"] == "nsfw"

	db, err := db()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Print(err)
		return
	}
	var url URL
	err = db.Get(&url, "SELECT * FROM urls WHERE nsfw=$1 ORDER BY random() LIMIT 1", nsfw)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Print(err)
		return
	}

	b, err := json.MarshalIndent(url, " ", "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Print(err)
		return
	}
	w.Write(b)
}

func serveAsset(r *mux.Router, assetPath string) {
	r.HandleFunc(assetPath, func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, path.Join(".", assetPath))
	})
}

func updateSubReddit(name string) error {
	log.Printf("Downloading %q...\n", name)
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

			exists, err := existsInDB(redditURL.URL, sourceURL)
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
				NSFW:      redditURL.Over18,
				Title:     redditURL.Title,
				SourceURL: sourceURL,
				URL:       redditURL.URL,
				WebMURL:   information.WebMURL,
				MP4URL:    information.MP4URL,
				Width:     information.Width,
				Height:    information.Height,
				CreatedAt: time.Unix(int64(redditURL.Created), 0),
			}
			oldestAllowed := time.Now().AddDate(-1, 0, 0)
			if url.CreatedAt.Before(oldestAllowed) {
				log.Printf("Not saving url with date %v\n", url.CreatedAt)
			} else {
				err = saveURL(url)
				if err != nil {
					log.Println(err)
				}
			}
		}
		time.Sleep(2 * time.Second)
	}
}

func updateRedditForever() {
	reddits := []string{
		// sfw
		"gifs", "perfectloops", "noisygifs", "analogygifs",
		"reversegif", "funny_gifs", "funnygifs", "aww_gifs", "wheredidthesodago",
		"AnimalsBeingJerks", "AnimalGIFs", "birdreactiongifs", "CatGifs", "catreactiongifs",
		"Puggifs", "KimJongUnGifs", "SpaceGifs", "physicsgifs", "educationalgifs",
		"chemicalreactiongifs", "mechanical_gifs", "shockwaveporn",
		// nsfw
		"gifsgonewild", "porn_gifs", "PornGifs", "NSFW_SEXY_GIF", "nsfwcelebgifs",
		"adultgifs", "NSFW_GIF", "nsfw_gifs", "porngif", "randomsexygifs", "TittyDrop",
		"boobbounce", "boobgifs", "celebgifs",
	}
	for i := range reddits {
		j := rand.Intn(i + 1)
		reddits[i], reddits[j] = reddits[j], reddits[i]
	}

	go func() {
		for {
			for _, s := range reddits {
				err := updateSubReddit(s)
				if err != nil {
					log.Println(err)
				}
			}

			err := dropOlderGifs()
			if err != nil {
				log.Println(err)
			}
		}
	}()
}

func getURLs(query string, nsfw bool, page int, pageSize int) ([]URL, error) {
	db, err := db()
	if err != nil {
		return nil, err
	}

	var rows *sqlx.Rows
	if query == "" {
		rows, err = db.Queryx(`
	SELECT * FROM urls
		WHERE nsfw = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`,
			nsfw, pageSize, (page-1)*pageSize)
	} else {
		rows, err = db.Queryx(`
	SELECT * FROM urls,
		to_tsquery('pg_catalog.english', $1) AS query
		WHERE nsfw=$2
		AND (tsv @@ query)
		ORDER BY ts_rank_cd(tsv, query) DESC
		LIMIT $3 OFFSET $4`,
			query, nsfw, pageSize, (page-1)*pageSize)
	}

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

func existsInDB(url string, sourceURL string) (bool, error) {
	db, err := db()
	if err != nil {
		return false, err
	}

	var count int
	err = db.Get(&count, "SELECT count(*) FROM urls WHERE url = $1 OR source_url = $2;", url, sourceURL)
	if err != nil {
		return false, err
	}
	return count != 0, nil
}

func dropOlderGifs() error {
	db, err := db()
	if err != nil {
		return err
	}

	_, err = db.Exec("DELETE FROM urls WHERE created_at < now() - interval '1 years'")
	return err
}

func saveURL(url URL) error {
	db, err := db()
	if err != nil {
		return err
	}

	_, err = db.Exec(`
	INSERT INTO urls (
		created_at, title, nsfw, url, source_url, webmurl, mp4url, width, height
	) VALUES (
		$1, $2, $3, $4, $5, $6, $7, $8, $9
	)`,
		url.CreatedAt,
		url.Title,
		url.NSFW,
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
	err := mig.Migrate("postgres", databaseURL(), "./migrations")
	if err != nil {
		log.Fatal(err)
	}

	updateRedditForever()

	port := flag.String("port", "8080", "the port to bind to")
	flag.Parse()

	r := mux.NewRouter()
	serveAsset(r, "/assets/styles/main.css")
	serveAsset(r, "/assets/styles/items.css")
	serveAsset(r, "/assets/scripts/gifs.js")
	serveAsset(r, "/assets/scripts/packery.pkgd.min.js")
	serveAsset(r, "/assets/images/loading.gif")
	r.Handle("/", handlers.LoggingHandler(os.Stdout, http.HandlerFunc(handler)))
	r.Handle("/api/random/{work}", handlers.LoggingHandler(os.Stdout, http.HandlerFunc(apiRandomHandler)))
	r.Handle("/{work}", handlers.LoggingHandler(os.Stdout, http.HandlerFunc(handler)))

	http.Handle("/", r)
	fmt.Printf("Starting on port %v\n", *port)
	err = http.ListenAndServe("0.0.0.0:"+*port, nil)
	log.Fatal(err)
}

var database *sqlx.DB

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
