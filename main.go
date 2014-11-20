package main

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"flag"
	"fmt"
	"github.com/AndrewVos/ancientcitadel/reddit"
	"github.com/golang/groupcache"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"image/gif"
	"image/jpeg"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"runtime"
	"strconv"
	"sync"
	"text/template"
	"time"
)

var cacher *groupcache.Group
var mutex sync.Mutex
var urls map[string][]*URL

type Page struct {
	URLs         []*URL
	CurrentPage  int
	NextPagePath string
}

type URL struct {
	Title   string
	URL     string
	Preview string
}

func getImage(ctx groupcache.Context, key string, dest groupcache.Sink) error {
	cachePath := fmt.Sprintf("%x", md5.Sum([]byte(key)))
	cachePath = path.Join("cache", cachePath)

	if _, err := os.Stat(cachePath); err == nil {
		b, err := ioutil.ReadFile(cachePath)
		if err != nil {
			return err
		}
		dest.SetBytes(b)
	} else {
		response, err := http.Get(key)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		image, err := gif.Decode(response.Body)
		if err != nil {
			return err
		}

		buffer := &bytes.Buffer{}
		err = jpeg.Encode(buffer, image, &jpeg.Options{50})
		if err != nil {
			return err
		}

		err = os.MkdirAll("cache", 0700)
		if err != nil {
			return err
		}
		bytes := buffer.Bytes()
		err = ioutil.WriteFile(cachePath, bytes, 0700)
		if err != nil {
			return err
		}
		dest.SetBytes(bytes)
	}
	return nil
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

	for _, url := range pageOfURLs {
		if url.Preview == "" {
			var buffer []byte
			err := cacher.Get(nil, url.URL, groupcache.AllocatingByteSliceSink(&buffer))
			if err != nil {
				log.Print(err)
				continue
			}
			url.Preview = fmt.Sprintf("data:%s;base64,%s", "image/gif", base64.StdEncoding.EncodeToString(buffer))
		}
	}
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

			newURLs[redditURL.Work] = append(newURLs[redditURL.Work], &URL{
				Title: redditURL.Title,
				URL:   redditURL.URL,
			})
		}
		mutex.Lock()
		urls = newURLs
		mutex.Unlock()
	}

	updateReddit()
	ticker := time.NewTicker(60 * time.Second)
	go func() {
		for _ = range ticker.C {
			updateReddit()
		}
	}()
}

func main() {
	runtime.GOMAXPROCS(4)

	updateRedditForever()

	port := flag.String("port", "8080", "the port to bind to")
	flag.Parse()

	cacher = groupcache.NewGroup("gifs", 128<<20, groupcache.GetterFunc(getImage))
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
