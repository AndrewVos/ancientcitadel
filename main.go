package main

import (
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/AndrewVos/ancientcitadel/gfycat"
	"github.com/AndrewVos/ancientcitadel/reddit"
	"github.com/AndrewVos/mig"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

const (
	PageSize = 20
)

type Page struct {
	URLs        []URL
	URL         URL
	CurrentPage int
	NextPage    *url.URL
	Query       string
	NSFW        bool
	AgeVerified bool
	Top         bool
}

func (p Page) RequiresAgeVerification() bool {
	return p.NSFW && p.AgeVerified == false
}

func NewPageFromRequest(w http.ResponseWriter, r *http.Request) (Page, error) {
	page := Page{CurrentPage: 1}
	if p := r.URL.Query().Get("page"); p != "" {
		page.CurrentPage, _ = strconv.Atoi(p)
	}

	page.Query = r.URL.Query().Get("q")
	page.Top = mux.Vars(r)["top"] == "top"
	page.NSFW = mux.Vars(r)["work"] == "nsfw"

	id := mux.Vars(r)["id"]
	if id != "" {
		url, err := getURL(id)
		if err != nil {
			return Page{}, err
		}
		page.URL = url
		page.NSFW = url.NSFW
		err = storeURLView(url)
		if err != nil {
			return Page{}, err
		}
	} else if page.Top {
		urls, err := getTopURLs(page.NSFW, page.CurrentPage, PageSize)
		if err != nil {
			return Page{}, err
		}
		page.URLs = urls
	} else {
		urls, err := getURLs(page.Query, page.NSFW, page.CurrentPage, PageSize)
		if err != nil {
			return Page{}, err
		}
		page.URLs = urls
	}

	page.NextPage, _ = url.Parse("")
	q := page.NextPage.Query()
	if page.Query != "" {
		q.Set("q", page.Query)
	}
	q.Set("page", fmt.Sprintf("%v", page.CurrentPage+1))
	page.NextPage.RawQuery = q.Encode()

	if r.Method == "POST" {
		r.ParseForm()
		if r.FormValue("age-verified") == "yes" {
			expiration := time.Now().Add(365 * 24 * time.Hour)
			cookie := http.Cookie{
				Name:    "age-verified",
				Value:   "yes",
				Expires: expiration,
				Path:    "/",
			}
			http.SetCookie(w, &cookie)
			page.AgeVerified = true
		}
	} else {
		ageVerified, err := r.Cookie("age-verified")
		if err == nil {
			page.AgeVerified = ageVerified.Value == "yes"
		}
	}

	return page, nil
}

type URL struct {
	ID        int       `db:"id"`
	CreatedAt time.Time `db:"created_at"`
	Title     string    `db:"title"`
	SourceURL string    `db:"source_url"`
	URL       string    `db:"url"`
	GfyName   string    `db:"gfy_name"`
	WebMURL   string    `db:"webmurl"`
	MP4URL    string    `db:"mp4url"`
	Width     int       `db:"width"`
	Height    int       `db:"height"`
	NSFW      bool      `db:"nsfw"`
	Views     int       `db:"views"`

	// never used, just here to appease sqlx
	TSV   string `db:"tsv" json:"-"`
	Query string `db:"query" json:"-"`
}

func (u URL) ToJSON() (string, error) {
	b, err := json.MarshalIndent(u, " ", "")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (u URL) HomeURL() string {
	return fmt.Sprintf("/gif/%v", u.ID)
}

func (u URL) ShareMarkdown() string {
	return fmt.Sprintf("![%s](%s)", u.URL, u.URL)
}

func (u URL) Thumbnail() string {
	return fmt.Sprintf("http://thumbs.gfycat.com/%s-poster.jpg", u.GfyName)
}

func templates(layout string) []string {
	defaultTemplates := []string{
		"navigation.html",
		"head.html",
		"google-analytics.html",
		"gif-item.html",
		"age-verification.html",
	}
	templates := []string{layout}
	for _, template := range defaultTemplates {
		templates = append(templates, template)
	}
	return templates
}

func genericHandler(layout string, w http.ResponseWriter, r *http.Request) {
	template, err := template.ParseFiles(templates(layout)...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Print(err)
		return
	}
	page, err := NewPageFromRequest(w, r)
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

func gifHandler(w http.ResponseWriter, r *http.Request) {
	genericHandler("gif.html", w, r)
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	genericHandler("index.html", w, r)
}

func sitemapHandler(w http.ResponseWriter, r *http.Request) {
	db, err := db()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Print(err)
		return
	}

	gzip := gzip.NewWriter(w)
	defer gzip.Close()

	gzip.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>` + "\n"))
	gzip.Write([]byte(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n"))
	offset := 0
	for {
		var urls []URL
		err = db.Select(&urls, `
		SELECT * FROM urls
			WHERE nsfw = $1
			ORDER BY created_at DESC
			OFFSET $2
			LIMIT 50
		`, false, offset)

		if len(urls) == 0 {
			break
		}
		for _, url := range urls {
			gzip.Write([]byte(fmt.Sprintf("  <url><loc>http://ancientcitadel.com%v</loc></url>\n", url.HomeURL())))
		}
		offset += 1
	}
	gzip.Write([]byte("</urlset>\n"))
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

func serveAsset(r *mux.Router, asset string) {
	r.HandleFunc(asset, func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, path.Join(".", asset))
	})
}

func serveAssets(r *mux.Router) {
	paths := []string{"styles", "scripts", "favicons"}
	for _, assetSubDirectory := range paths {
		files, err := ioutil.ReadDir(path.Join("assets", assetSubDirectory))
		if err != nil {
			log.Fatal(err)
		}
		for _, file := range files {
			serveAsset(r, "/"+path.Join("assets", assetSubDirectory, file.Name()))
		}
	}
}

func validGIFURL(url string) bool {
	if strings.HasSuffix(url, ".jpg") {
		return false
	}
	if strings.Contains(url, "youtube.com") {
		return false
	}

	if strings.Contains(url, "imgur.com") {
		return true
	}
	if strings.Contains(url, "gfycat.com") {
		return true
	}
	if strings.HasSuffix(url, ".gif") {
		return true
	}
	return false
}

func updateSubReddit(name string) error {
	subReddit := reddit.SubReddit{Name: name}

	fmt.Printf("downloading /r/%v...\n", name)
	for {
		redditURLs, err := subReddit.NextPage()
		if err != nil {
			return err
		}
		if len(redditURLs) == 0 {
			return nil
		}
		for _, redditURL := range redditURLs {
			if validGIFURL(redditURL.URL) == false {
				continue
			}

			if strings.Contains(redditURL.URL, "imgur.com") && !strings.HasSuffix(redditURL.URL, ".gif") {
				redditURL.URL = strings.TrimSuffix(redditURL.URL, ".gifv")
				redditURL.URL = redditURL.URL + ".gif"
			}
			if strings.Contains(redditURL.URL, "gfycat.com") && !strings.HasSuffix(redditURL.URL, ".gif") {
				redditURL.URL = strings.Replace(redditURL.URL, "http://gfycat", "http://giant.gfycat", -1)
				redditURL.URL += ".gif"
			}

			sourceURL := "https://reddit.com" + redditURL.Permalink

			url := URL{
				Title:     redditURL.Title,
				NSFW:      redditURL.Over18,
				SourceURL: sourceURL,
				URL:       redditURL.URL,
				CreatedAt: time.Unix(int64(redditURL.Created), 0),
			}

			id, err := existsInDB(url)
			if err != nil {
				log.Println(err)
				continue
			}

			if id != 0 {
				updateURL(id, url)
				continue
			}

			information, err := gfycat.Gif(redditURL.URL)
			if err != nil {
				log.Println(err)
				continue
			}

			url.GfyName = information.GfyName
			url.WebMURL = information.WebMURL
			url.MP4URL = information.MP4URL
			url.Width = information.Width
			url.Height = information.Height

			err = saveURL(url)
			if err != nil {
				log.Println(err)
				continue
			}
		}
		time.Sleep(2 * time.Second)
	}
}

func updateRedditForever() {
	reddits := []string{
		// sfw
		"gifs", "perfectloops", "noisygifs", "analogygifs",
		"reversegif", "aww_gifs", "SlyGifs",
		"AnimalsBeingJerks", "shittyreactiongifs", "CatGifs",
		"Puggifs", "SpaceGifs", "physicsgifs", "educationalgifs", "shockwaveporn",
		// nsfw
		"gifsgonewild", "porn_gifs", "PornGifs", "NSFW_SEXY_GIF", "nsfwcelebgifs",
		"adultgifs", "NSFW_GIF", "nsfw_gifs", "porngif", "randomsexygifs",
	}

	for i := range reddits {
		rand.Seed(time.Now().UnixNano())
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
		}
	}()
}

func storeURLView(url URL) error {
	db, err := db()
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO url_views (url_id) VALUES ($1)`, url.ID)
	return err
}

func getURL(id string) (URL, error) {
	db, err := db()
	if err != nil {
		return URL{}, err
	}
	var url URL
	err = db.Get(&url, `SELECT * FROM urls WHERE id = $1 LIMIT 1`, id)
	return url, err
}

func getTopURLs(nsfw bool, page int, pageSize int) ([]URL, error) {
	db, err := db()
	if err != nil {
		return nil, err
	}

	var rows *sqlx.Rows

	rows, err = db.Queryx(`
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

func getURLs(query string, nsfw bool, page int, pageSize int) ([]URL, error) {
	db, err := db()
	if err != nil {
		return nil, err
	}

	var rows *sqlx.Rows
	if query != "" {
		wordFinder := regexp.MustCompile("\\w+")
		queryParts := wordFinder.FindAllString(query, -1)
		tSearchQuery := strings.Join(queryParts, "&")

		rows, err = db.Queryx(`
	SELECT * FROM urls,
		to_tsquery('pg_catalog.english', $1) AS query
		WHERE nsfw=$2
		AND (tsv @@ query)
		ORDER BY ts_rank_cd(tsv, query) DESC
		LIMIT $3 OFFSET $4`,
			tSearchQuery, nsfw, pageSize, (page-1)*pageSize)
	} else {
		rows, err = db.Queryx(`
	SELECT * FROM urls
		WHERE nsfw = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`,
			nsfw, pageSize, (page-1)*pageSize)
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

func existsInDB(url URL) (int, error) {
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

func updateURL(id int, url URL) error {
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

func saveURL(url URL) error {
	db, err := db()
	if err != nil {
		return err
	}

	_, err = db.Exec(`
	INSERT INTO urls (
		created_at, title, gfy_name, nsfw, url, source_url, webmurl, mp4url, width, height
	) VALUES (
		$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
	)`,
		url.CreatedAt,
		url.Title,
		url.GfyName,
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

	port := flag.String("port", "8080", "the port to bind to")
	flag.Parse()

	r := mux.NewRouter()
	serveAssets(r)
	r.Handle("/api/random/{work:nsfw|sfw}", handlers.LoggingHandler(os.Stdout, http.HandlerFunc(apiRandomHandler)))
	r.Handle("/", handlers.LoggingHandler(os.Stdout, http.HandlerFunc(mainHandler)))
	r.Handle("/{top:top}", handlers.LoggingHandler(os.Stdout, http.HandlerFunc(mainHandler)))
	r.Handle("/{work:nsfw}", handlers.LoggingHandler(os.Stdout, http.HandlerFunc(mainHandler)))
	r.Handle("/{work:nsfw}/{top:top}", handlers.LoggingHandler(os.Stdout, http.HandlerFunc(mainHandler)))
	r.Handle("/gif/{id:\\d+}", handlers.LoggingHandler(os.Stdout, http.HandlerFunc(gifHandler)))

	r.Handle("/sitemap.xml.gz", handlers.LoggingHandler(os.Stdout, http.HandlerFunc(sitemapHandler)))

	http.Handle("/", r)
	fmt.Printf("Starting on port %v...\n", *port)

	updateRedditForever()
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
