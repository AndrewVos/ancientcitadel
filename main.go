package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/golang/groupcache"
	"github.com/gorilla/mux"
	"image/gif"
	"image/jpeg"
	"io/ioutil"
	"log"
	"net/http"
	"text/template"
)

type RedditResponse struct {
	Data RedditResponseData
}

type RedditResponseData struct {
	After    string
	Children []RedditResponseChild
}

type RedditResponseChild struct {
	Data RedditResponseChildData
}

type RedditResponseChildData struct {
	Title string
	URL   string
}

type Page struct {
	SubReddit string
	After     string
	URLS      []URL
}

type URL struct {
	Title string
	URL   string
}

func getImage(ctx groupcache.Context, key string, dest groupcache.Sink) error {
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
	err = jpeg.Encode(buffer, image, &jpeg.Options{jpeg.DefaultQuality})
	if err != nil {
		return err
	}
	dest.SetBytes(buffer.Bytes())
	return nil
}

func redditPage(subReddit string, after string) (Page, error) {
	url := fmt.Sprintf("https://api.reddit.com/r/%v/hot.json", subReddit)
	if after != "" {
		url += "?after=" + after
	}

	response, err := http.Get(url)
	if err != nil {
		return Page{}, err
	}
	b, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return Page{}, err
	}
	var redditResponse RedditResponse
	err = json.Unmarshal(b, &redditResponse)
	if err != nil {
		return Page{}, err
	}

	page := Page{}
	page.SubReddit = subReddit
	page.After = redditResponse.Data.After
	for _, child := range redditResponse.Data.Children {
		page.URLS = append(page.URLS, URL{
			Title: child.Data.Title,
			URL:   child.Data.URL,
		})
	}
	return page, nil
}

func root(w http.ResponseWriter, r *http.Request) {
	template, err := template.ParseFiles("index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Print(err)
		return
	}
	err = template.Execute(w, SubReddits())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Print(err)
		return
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	template, err := template.ParseFiles("page.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Print(err)
		return
	}
	page, err := redditPage(mux.Vars(r)["subreddit"], mux.Vars(r)["after"])
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

func main() {
	port := flag.String("port", "8080", "the port to bind to")
	flag.Parse()

	cacher = groupcache.NewGroup("gifs", 128<<20, groupcache.GetterFunc(getImage))
	r := mux.NewRouter()
	r.HandleFunc("/", root)
	r.HandleFunc("/r/{subreddit}", handler)
	r.HandleFunc("/r/{subreddit}/{after}", handler)
	r.HandleFunc("/gif", gifFrame)
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./public/")))
	http.Handle("/", r)
	err := http.ListenAndServe(":"+*port, nil)
	log.Fatal(err)
}
