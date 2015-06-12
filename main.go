package main

import (
	"compress/gzip"
	"encoding/base64"
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
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/AndrewVos/ancientcitadel/gfycat"
	"github.com/AndrewVos/ancientcitadel/minify"
	"github.com/AndrewVos/ancientcitadel/reddit"
	"github.com/AndrewVos/mig"
	"github.com/ChimeraCoder/anaconda"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/mrjones/oauth"
	"github.com/nytimes/gziphandler"
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
	w.Header().Set("Content-Type", "text/html")

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

func twitterCallbackHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	c := oauth.NewConsumer(
		os.Getenv("TWITTER_CONSUMER_KEY"),
		os.Getenv("TWITTER_CONSUMER_SECRET"),
		oauth.ServiceProvider{
			RequestTokenUrl:   "https://api.twitter.com/oauth/request_token",
			AuthorizeTokenUrl: "https://api.twitter.com/oauth/authorize",
			AccessTokenUrl:    "https://api.twitter.com/oauth/access_token",
		})

	values := r.URL.Query()
	verificationCode := values.Get("oauth_verifier")
	tokenKey := values.Get("oauth_token")

	accessToken, err := c.AuthorizeToken(tokens[tokenKey], verificationCode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Print(err)
		return
	}

	expiration := time.Now().Add(365 * 24 * time.Hour)
	http.SetCookie(w, &http.Cookie{
		Name:    "twitter_access_token",
		Value:   accessToken.Token,
		Expires: expiration,
		Path:    "/",
	})
	http.SetCookie(w, &http.Cookie{
		Name:    "twitter_secret",
		Value:   accessToken.Secret,
		Expires: expiration,
		Path:    "/",
	})

	w.Write([]byte("<script>window.close();</script>"))
}

var tokens = map[string]*oauth.RequestToken{}

func tweetHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	var twitterToken string
	var twitterSecret string

	if c, err := r.Cookie("twitter_access_token"); err == nil {
		twitterToken = c.Value
	}
	if c, err := r.Cookie("twitter_secret"); err == nil {
		twitterSecret = c.Value
	}

	c := oauth.NewConsumer(
		os.Getenv("TWITTER_CONSUMER_KEY"),
		os.Getenv("TWITTER_CONSUMER_SECRET"),
		oauth.ServiceProvider{
			RequestTokenUrl:   "https://api.twitter.com/oauth/request_token",
			AuthorizeTokenUrl: "https://api.twitter.com/oauth/authorize",
			AccessTokenUrl:    "https://api.twitter.com/oauth/access_token",
		})

	if twitterToken == "" || twitterSecret == "" {
		token, requestURL, err := c.GetRequestTokenAndUrl("http://ancientcitadel.com/twitter/callback")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Print(err)
			return
		}
		tokens[token.Token] = token
		http.Redirect(w, r, requestURL, http.StatusTemporaryRedirect)
	} else {
		id := mux.Vars(r)["id"]
		gif, err := getURL(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Print(err)
			return
		}

		anaconda.SetConsumerKey(os.Getenv("TWITTER_CONSUMER_KEY"))
		anaconda.SetConsumerSecret(os.Getenv("TWITTER_CONSUMER_SECRET"))
		api := anaconda.NewTwitterApi(twitterToken, twitterSecret)

		gifResponse, err := http.Get(gif.URL)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Print(err)
			return
		}
		defer gifResponse.Body.Close()

		b, err := ioutil.ReadAll(gifResponse.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Print(err)
			return
		}
		base64Encoded := base64.StdEncoding.EncodeToString(b)
		media, err := api.UploadMedia(base64Encoded)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Print(err)
			return
		}

		v := url.Values{}
		v.Set("media_ids", media.MediaIDString)
		_, err = api.PostTweet("http://ancientcitadel.com/gif/"+id, v)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Print(err)
			return
		}
	}
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

	_, err = gzip.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" +
		`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Print(err)
		return
	}

	offset := 0
	limit := 1000
	for {
		var ids []int

		err = db.Select(&ids, `
		SELECT id FROM urls
			WHERE nsfw = $1
			ORDER BY created_at DESC
			OFFSET $2
			LIMIT $3
		`, false, offset, limit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Print(err)
			return
		}

		if len(ids) == 0 {
			break
		}

		for _, id := range ids {
			_, err := gzip.Write([]byte(fmt.Sprintf("  <url><loc>http://ancientcitadel.com/gif/%v</loc></url>\n", id)))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				log.Print(err)
				return
			}
		}
		offset += limit
	}
	gzip.Write([]byte("</urlset>\n"))
}

func apiRandomHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

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

func updateSubReddit(name string, nsfw bool) error {
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
				redditURL.URL = strings.TrimSuffix(redditURL.URL, ".webm")
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

			if nsfw != url.NSFW {
				continue
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
	redditTypes := map[string][]string{
		"sfw": []string{
			"gifs", "perfectloops", "noisygifs", "analogygifs",
			"reversegif", "aww_gifs", "SlyGifs",
			"AnimalsBeingJerks", "shittyreactiongifs", "CatGifs",
			"Puggifs", "SpaceGifs", "physicsgifs", "educationalgifs", "shockwaveporn",
		},
		"nsfw": []string{
			"gifsgonewild", "porn_gifs", "PornGifs", "NSFW_SEXY_GIF", "nsfwcelebgifs",
			"adultgifs", "NSFW_GIF", "nsfw_gifs", "porngif", "randomsexygifs",
		},
	}

	shuffle := func(s []string) {
		for i := range s {
			rand.Seed(time.Now().UnixNano())
			j := rand.Intn(i + 1)
			s[i], s[j] = s[j], s[i]
		}
	}

	shuffle(redditTypes["sfw"])
	shuffle(redditTypes["nsfw"])

	go func() {
		for {
			for _, s := range redditTypes["sfw"] {
				err := updateSubReddit(s, false)
				if err != nil {
					log.Println(err)
				}
			}
			for _, s := range redditTypes["nsfw"] {
				err := updateSubReddit(s, true)
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

func addHandlerWithoutGZIP(path string, r *mux.Router, h http.Handler) {
	h = handlers.LoggingHandler(os.Stdout, h)
	r.Handle(path, h)
}

func addHandler(path string, r *mux.Router, h http.Handler) {
	h = gziphandler.GzipHandler(h)
	h = handlers.LoggingHandler(os.Stdout, h)
	r.Handle(path, h)
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
	css := []string{
		"assets/styles/bootstrap.min.css",
		"assets/styles/main.css",
		"assets/styles/remodal.css",
		"assets/styles/remodal-default-theme.css",
	}
	js := []string{
		"assets/scripts/jquery.min.js",
		"assets/scripts/remodal.min.js",
		"assets/scripts/tweet.js",
		"assets/scripts/navigation.js",
		"assets/scripts/pack.js",
		"assets/scripts/gifs.js",
	}

	addHandler("/styles/all.css", r, minify.CSSHandler(css))
	addHandler("/scripts/all.js", r, minify.JSHandler(js))

	addHandler("/assets/favicons/", r, http.StripPrefix("/assets/favicons/", http.FileServer(http.Dir("assets/favicons"))))

	addHandler("/api/random/{work:nsfw|sfw}", r, http.HandlerFunc(apiRandomHandler))
	addHandler("/", r, http.HandlerFunc(mainHandler))
	addHandler("/{top:top}", r, http.HandlerFunc(mainHandler))
	addHandler("/{work:nsfw}", r, http.HandlerFunc(mainHandler))
	addHandler("/{work:nsfw}/{top:top}", r, http.HandlerFunc(mainHandler))
	addHandler("/gif/{id:\\d+}", r, http.HandlerFunc(gifHandler))

	addHandler("/tweet/{id:\\d+}", r, http.HandlerFunc(tweetHandler))
	addHandler("/twitter/callback", r, http.HandlerFunc(twitterCallbackHandler))

	addHandlerWithoutGZIP("/sitemap.xml.gz", r, http.HandlerFunc(sitemapHandler))

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
