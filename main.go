package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"io/ioutil"
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
	After string
	URLS  []URL
}

type URL struct {
	Title string
	URL   string
}

func redditPage(after string) (Page, error) {
	subReddit := "gifs"
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
	page.After = redditResponse.Data.After
	for _, child := range redditResponse.Data.Children {
		page.URLS = append(page.URLS, URL{
			Title: child.Data.Title,
			URL:   child.Data.URL,
		})
	}
	return page, nil
}

func handler(w http.ResponseWriter, r *http.Request) {
	template, err := template.ParseFiles("index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = template.Execute(w, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func page(w http.ResponseWriter, r *http.Request) {
	template, err := template.ParseFiles("page.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	urls, err := redditPage(mux.Vars(r)["after"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	err = template.Execute(w, urls)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/", handler)
	r.HandleFunc("/page", page)
	r.HandleFunc("/page/{after}", page)
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./public/")))
	http.Handle("/", r)
	http.ListenAndServe(":8080", nil)
}
