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
	"text/template"
	"time"
)

var cacher *groupcache.Group

type Page struct {
	URLs         []URL
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

	for _, redditURL := range reddit.SubRedditURLs(work, page.CurrentPage, 20) {
		var buffer []byte
		err := cacher.Get(nil, redditURL.URL, groupcache.AllocatingByteSliceSink(&buffer))
		if err != nil {
			log.Print(err)
			continue
		}
		preview := fmt.Sprintf("data:%s;base64,%s", "image/gif", base64.StdEncoding.EncodeToString(buffer))
		page.URLs = append(page.URLs, URL{
			Title:   redditURL.Title,
			URL:     redditURL.URL,
			Preview: preview,
		})
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
	reddit.UpdateRedditData()
	ticker := time.NewTicker(60 * time.Second)
	go func() {
		for _ = range ticker.C {
			reddit.UpdateRedditData()
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
