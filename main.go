package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/AndrewVos/ancientcitadel/reddit"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"runtime"
	"strconv"
	"sync"
	"text/template"
	"time"
)

var mutex sync.Mutex
var urls map[string][]*URL

type Page struct {
	URLs         []*URL
	CurrentPage  int
	NextPagePath string
}

type URL struct {
	Title string
	URL   string
	Data  GfyCatInformation
}

type GfyCatInformation struct {
	WebMURL string
	Width   int
	Height  int
}

func GetGfyCatInformation(gifURL string) (GfyCatInformation, error) {
	type UploadedGif struct {
		Error   string `json:"error"`
		GfyName string `json:"gfyname"`
		WebMURL string `json:"webmUrl"`
	}
	type GfyItem struct {
		Width  int
		Height int
	}

	uploadURL := fmt.Sprintf("http://upload.gfycat.com/transcode?fetchUrl=%v", url.QueryEscape(gifURL))
	uploadResponse, err := http.Get(uploadURL)
	if err != nil {
		return GfyCatInformation{}, errors.New(fmt.Sprintf("couldn't upload gif to gfycat, but got this?\n%v\n", err))
	}
	defer uploadResponse.Body.Close()
	b, err := ioutil.ReadAll(uploadResponse.Body)
	if err != nil {
		return GfyCatInformation{}, errors.New(fmt.Sprintf("Couldn't read body from %v", uploadURL))
	}
	var uploadedGif UploadedGif
	err = json.Unmarshal(b, &uploadedGif)

	if err != nil {
		return GfyCatInformation{}, errors.New(
			fmt.Sprintf("couldn't decode this from gfycat:\n%v\nError:\n%v\nURL: %v", string(b), err, uploadURL),
		)
	}
	if uploadedGif.Error != "" {
		return GfyCatInformation{}, errors.New(fmt.Sprintf("got error from url %q\n%v", uploadURL, uploadedGif.Error))
	}

	getURL := fmt.Sprintf("http://gfycat.com/cajax/get/%v", uploadedGif.GfyName)
	getResponse, err := http.Get(getURL)
	if err != nil {
		return GfyCatInformation{}, errors.New(fmt.Sprintf("couldn't get gif from gfycat, but got this?\n%v\n", err))
	}

	b, err = ioutil.ReadAll(getResponse.Body)
	if err != nil {
		return GfyCatInformation{}, errors.New(fmt.Sprintf("Couldn't read body from %v", getURL))
	}
	defer getResponse.Body.Close()

	var j map[string]GfyItem
	err = json.Unmarshal(b, &j)
	if err != nil {
		return GfyCatInformation{}, errors.New(
			fmt.Sprintf("couldn't decode this from gfycat:\n%v\nError:\n%v\nURL: %v", string(b), err, getURL),
		)
	}

	return GfyCatInformation{
		WebMURL: uploadedGif.WebMURL,
		Width:   j["gfyItem"].Width,
		Height:  j["gfyItem"].Height,
	}, nil
}

func getPageOfURLs(work string, page int, pageSize int) []*URL {
	mutex.Lock()
	workURLs := urls[work]
	mutex.Unlock()

	startIndex := (page - 1) * pageSize
	endIndex := startIndex + pageSize
	if startIndex >= len(workURLs) {
		return nil
	}
	if endIndex >= len(workURLs) {
		endIndex = len(workURLs) - 1
	}
	pageOfURLs := workURLs[startIndex:endIndex]
	return pageOfURLs
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

	page.URLs = getPageOfURLs(work, page.CurrentPage, 20)
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
		newURLs := map[string][]*URL{}

		client := http.Client{}
		for _, redditURL := range redditURLs {
			request, err := http.NewRequest("HEAD", redditURL.URL, nil)
			if err != nil {
				log.Println(err)
				continue
			}
			response, err := client.Do(request)
			if err != nil {
				log.Println(err)
				continue
			}
			contentType := response.Header.Get("Content-Type")
			if contentType != "image/gif" {
				log.Printf("%v wasn't image/gif, was (%v)\n", redditURL.URL, contentType)
				continue
			}

			information, err := GetGfyCatInformation(redditURL.URL)
			if err != nil {
				log.Println(err)
				continue
			}
			newURLs[redditURL.Work] = append(newURLs[redditURL.Work], &URL{
				Title: redditURL.Title,
				URL:   redditURL.URL,
				Data:  information,
			})
		}
		mutex.Lock()
		urls = newURLs
		mutex.Unlock()
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

func main() {
	runtime.GOMAXPROCS(4)

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
