package main

import (
	"bytes"
	"crypto/md5"
	"flag"
	"fmt"
	"github.com/AndrewVos/ancientcitadel/reddit"
	"github.com/golang/groupcache"
	"github.com/gorilla/mux"
	"image/gif"
	"image/png"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"text/template"
)

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
		err = png.Encode(buffer, image)
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

type Page struct {
	URLs         []reddit.RedditURL
	CurrentPage  int
	NextPagePath string
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
	page.URLs = reddit.SubRedditURLs(work, page.CurrentPage, 20)
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

func gifFrame(w http.ResponseWriter, r *http.Request) {
	gifURL := r.URL.Query().Get("url")
	var buf []byte
	err := cacher.Get(nil, gifURL, groupcache.AllocatingByteSliceSink(&buf))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Print(err)
		return
	}
	w.Write(buf)
}

var cacher *groupcache.Group

func serveAsset(r *mux.Router, assetPath string) {
	r.HandleFunc(assetPath, func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, path.Join(".", assetPath))
	})
}

func main() {
	// runtime.GOMAXPROCS(4)
	reddit.UpdateRedditData()

	port := flag.String("port", "8080", "the port to bind to")
	flag.Parse()

	cacher = groupcache.NewGroup("gifs", 128<<20, groupcache.GetterFunc(getImage))
	r := mux.NewRouter()
	serveAsset(r, "/assets/styles/main.css")
	serveAsset(r, "/assets/styles/items.css")
	serveAsset(r, "/assets/scripts/gifs.js")
	r.HandleFunc("/", handler)
	r.HandleFunc("/gif-frame", gifFrame)
	r.HandleFunc("/{work}/{page}", handler)

	http.Handle("/", r)
	fmt.Printf("Starting on port %v\n", *port)
	err := http.ListenAndServe(":"+*port, nil)
	log.Fatal(err)
}
