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
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AndrewVos/ancientcitadel/assethandler"
	"github.com/AndrewVos/ancientcitadel/db"
	"github.com/AndrewVos/ancientcitadel/gifs"
	"github.com/AndrewVos/ancientcitadel/reddit"
	"github.com/AndrewVos/ancientcitadel/slug"
	"github.com/ChimeraCoder/anaconda"
	"github.com/dustin/go-humanize"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/justinas/alice"
	"github.com/mrjones/oauth"
	"github.com/nytimes/gziphandler"
)

const (
	PageSize = 20
)

type Page struct {
	URLs        []db.URL
	URL         db.URL
	CurrentPage int
	NextPage    string
	URLCount    string
	Query       string
	NSFW        bool
	AgeVerified bool
	New         bool
	Top         bool
	Shuffle     bool
}

func (p Page) RequiresAgeVerification() bool {
	return p.NSFW && p.AgeVerified == false
}

func NewPageFromRequest(w http.ResponseWriter, r *http.Request) (Page, error) {
	page := Page{CurrentPage: 1}
	if p := r.URL.Query().Get("page"); p != "" {
		page.CurrentPage, _ = strconv.Atoi(p)
	}

	count, err := db.GetURLCount()
	if err != nil {
		return Page{}, err
	}
	page.URLCount = fmt.Sprintf("%s", humanize.Comma(int64(count)))

	page.Query = r.URL.Query().Get("q")
	page.Top = mux.Vars(r)["top"] == "top"
	page.Shuffle = mux.Vars(r)["shuffle"] == "shuffle"
	page.New = !(page.Top || page.Shuffle)

	page.NSFW = mux.Vars(r)["work"] == "nsfw"

	gifSlug := mux.Vars(r)["slug"]
	var id int
	if gifSlug != "" {
		id, err = slug.Parse(gifSlug)
	}

	if id != 0 {
		url, err := db.GetURL(id)
		if err != nil {
			return Page{}, err
		}
		page.URL = url
		page.NSFW = url.NSFW
		err = db.StoreURLView(url)
		if err != nil {
			return Page{}, err
		}
	} else if page.Top {
		urls, err := db.GetTopURLs(page.NSFW, page.CurrentPage, PageSize)
		if err != nil {
			return Page{}, err
		}
		page.URLs = urls
	} else if page.Shuffle {
		urls, err := db.GetShuffledURLs(page.NSFW, page.CurrentPage, PageSize)
		if err != nil {
			return Page{}, err
		}
		page.URLs = urls
	} else {
		urls, err := db.GetURLs(page.Query, page.NSFW, page.CurrentPage, PageSize)
		if err != nil {
			return Page{}, err
		}
		page.URLs = urls
	}

	q := r.URL.Query()
	q.Set("page", fmt.Sprintf("%v", page.CurrentPage+1))
	page.NextPage = "?" + q.Encode()

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

func pageHandler(layout string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
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
		id, err := strconv.Atoi(mux.Vars(r)["id"])
		if err != nil {
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				log.Print(err)
				return
			}
		}

		gif, err := db.GetURL(id)
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
		_, err = api.PostTweet("http://ancientcitadel.com/"+gif.Permalink(), v)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Print(err)
			return
		}
	}
}

func sitemapHandler(w http.ResponseWriter, r *http.Request) {
	gzip := gzip.NewWriter(w)
	defer gzip.Close()

	_, err := gzip.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" +
		`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Print(err)
		return
	}

	page := 1
	for {
		var urls []db.URL

		urls, err := db.GetURLs("", false, page, 1000)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Print(err)
			return
		}

		if len(urls) == 0 {
			break
		}

		for _, url := range urls {
			_, err := gzip.Write([]byte(fmt.Sprintf("  <url><loc>http://ancientcitadel.com%v</loc></url>\n", url.Permalink())))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				log.Print(err)
				return
			}
		}
		page += 1
	}
	gzip.Write([]byte("</urlset>\n"))
}

func apiFeedHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	nsfw := mux.Vars(r)["work"] == "nsfw"
	order := mux.Vars(r)["order"]
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		page, _ = strconv.Atoi(p)
	}

	type JSONError struct {
		Error string `json:"error"`
	}

	var err error
	var urls []db.URL

	if order == "new" {
		urls, err = db.GetURLs("", nsfw, page, PageSize)
	} else if order == "top" {
		urls, err = db.GetTopURLs(nsfw, page, PageSize)
	}
	if err != nil {
		b, _ := json.Marshal(JSONError{Error: err.Error()})
		w.Write(b)
		return
	}

	if len(urls) == 0 {
		urls = []db.URL{}
	}

	b, err := json.Marshal(urls)
	if err != nil {
		b, _ := json.Marshal(JSONError{Error: err.Error()})
		w.Write(b)
		return
	}

	_, err = w.Write(b)
	if err != nil {
		b, _ := json.Marshal(JSONError{Error: err.Error()})
		w.Write(b)
		return
	}
}

func apiRandomHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	nsfw := mux.Vars(r)["work"] == "nsfw"

	url, err := db.GetRandomURL(nsfw)
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
	urlStorer := NewURLStorer()
	defer urlStorer.Wait()

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
				redditURL.URL = strings.Replace(redditURL.URL, "http://imgur.com", "http://i.imgur.com", -1)
				redditURL.URL = strings.Replace(redditURL.URL, "/gallery/", "/", -1)
				redditURL.URL = redditURL.URL + ".gif"
			}
			if strings.Contains(redditURL.URL, "gfycat.com") && !strings.HasSuffix(redditURL.URL, ".gif") {
				redditURL.URL = strings.Replace(redditURL.URL, "http://gfycat", "http://giant.gfycat", -1)
				redditURL.URL += ".gif"
			}

			sourceURL := "https://reddit.com" + redditURL.Permalink

			url := db.URL{
				Title:     redditURL.Title,
				NSFW:      redditURL.Over18,
				SourceURL: sourceURL,
				URL:       redditURL.URL,
				CreatedAt: time.Unix(int64(redditURL.CreatedUTC), 0),
			}

			if nsfw != url.NSFW {
				continue
			}

			id, err := db.ExistsInDB(url)
			if err != nil {
				log.Println(err)
				continue
			}

			if id != 0 {
				db.UpdateURL(id, url)
				continue
			}

			urlStorer.Upload(&url)
		}
	}
}

type URLStorer struct {
	urls      chan *db.URL
	waitGroup sync.WaitGroup
}

func (g *URLStorer) Upload(url *db.URL) {
	g.urls <- url
}

func (g *URLStorer) Wait() {
	close(g.urls)
	g.waitGroup.Wait()
}

func NewURLStorer() *URLStorer {
	uploader := &URLStorer{urls: make(chan *db.URL)}

	processors := 5
	for i := 1; i <= processors; i++ {
		uploader.waitGroup.Add(1)
		go func(processor int) {
			host := fmt.Sprintf("http://gifs%v.ancientcitadel.com", processor)

			for url := range uploader.urls {
				result, err := db.GetDownloadResult(url.URL)
				if err != nil {
					log.Println(err)
					continue
				}
				if result != nil && result.Success == false {
					continue
				}

				fmt.Printf("uploading %q to %q...\n", url.URL, host)
				information, err := gifs.Gif(host, url.URL)

				if err != nil {
					log.Println(err)
					err := db.StoreDownloadResult(url.URL, false)
					if err != nil {
						log.Println(err)
					}
					continue
				}
				err = db.StoreDownloadResult(url.URL, true)
				if err != nil {
					log.Println(err)
					continue
				}

				url.WEBMURL = information.WEBMURL
				url.MP4URL = information.MP4URL
				url.ThumbnailURL = information.JPGURL
				url.Width = information.Width
				url.Height = information.Height
				err = db.SaveURL(url)
				if err != nil {
					log.Println(err)
					continue
				}
			}
			uploader.waitGroup.Done()
		}(i)
	}
	return uploader
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
			"gifsgonewild", "porn_gifs", "PornGifs", "NSFW_SEXY_GIF",
			"adultgifs", "NSFW_GIF", "nsfw_gifs", "porngif",
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

func loggingHandler(next http.Handler) http.Handler {
	return handlers.LoggingHandler(os.Stdout, next)
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	runtime.GOMAXPROCS(4)
	err := db.Migrate()
	if err != nil {
		log.Fatal(err)
	}

	middleware := alice.New(
		loggingHandler,
		gziphandler.GzipHandler,
	)

	port := flag.String("port", "8080", "the port to bind to")
	flag.Parse()

	r := mux.NewRouter()

	jsHandler := assethandler.JS([]string{
		"assets/scripts/jquery.min.js",
		"assets/scripts/remodal.min.js",
		"assets/scripts/tweet.js",
		"assets/scripts/navigation.js",
		"assets/scripts/play-button.js",
		"assets/scripts/pack.js",
		"assets/scripts/gifs.js",
	})

	cssHandler := assethandler.CSS([]string{
		"assets/styles/bootstrap.min.css",
		"assets/styles/remodal.css",
		"assets/styles/remodal-default-theme.css",
		"assets/styles/main.css",
		"assets/styles/play-button.css",
	})
	handlers := map[string]http.Handler{
		"/compiled.js":            jsHandler,
		"/compiled.css":           cssHandler,
		"/assets/favicons/{icon}": http.StripPrefix("/assets/favicons/", http.FileServer(http.Dir("./assets/favicons/"))),
	}

	for path, handler := range handlers {
		r.Handle(path, middleware.Then(handler))
	}

	handlerFuncs := map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/random/{work:nsfw|sfw}":          apiRandomHandler,
		"/api/{work:nsfw|sfw}/{order:new|top}": apiFeedHandler,
		"/":                              pageHandler("index.html"),
		"/{top:top}":                     pageHandler("index.html"),
		"/{shuffle:shuffle}":             pageHandler("index.html"),
		"/{work:nsfw}":                   pageHandler("index.html"),
		"/{work:nsfw}/{top:top}":         pageHandler("index.html"),
		"/{work:nsfw}/{shuffle:shuffle}": pageHandler("index.html"),
		"/gif/{slug}":                    pageHandler("gif.html"),
		"/tweet/{id:\\d+}":               tweetHandler,
		"/twitter/callback":              twitterCallbackHandler,
		"/sitemap.xml.gz":                sitemapHandler,
	}

	for path, handlerFunc := range handlerFuncs {
		r.Handle(path, middleware.ThenFunc(handlerFunc))
	}

	http.Handle("/", r)
	fmt.Printf("Starting on port %v...\n", *port)

	updateRedditForever()
	err = http.ListenAndServe("0.0.0.0:"+*port, nil)
	log.Fatal(err)
}
