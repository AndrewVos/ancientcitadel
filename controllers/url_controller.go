package controllers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"text/template"

	"github.com/AndrewVos/ancientcitadel/db"
	"github.com/AndrewVos/ancientcitadel/slug"
	"github.com/dustin/go-humanize"
	"github.com/gorilla/mux"
)

const (
	PageSize = 20
)

type URLController struct{}

func NewURLController() *URLController {
	return &URLController{}
}

type Result struct {
	HumanCount          string
	SortByTop           bool
	SortByShuffle       bool
	SortByNew           bool
	NSFW                bool
	Query               string
	ShowAgeVerification bool
}

type IndexResult struct {
	Result
	CurrentPage  int
	NextPageLink string
	URLs         []db.URL
}

type ShowResult struct {
	Result
	URL db.URL
}

func writeError(err error, w http.ResponseWriter) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
	log.Println(err)
}

func (c *URLController) Show(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	template, err := template.ParseFiles(templates("gif.html")...)
	if err != nil {
		writeError(err, w)
		return
	}

	result := ShowResult{}

	gifSlug := mux.Vars(r)["slug"]
	id, err := slug.Parse(gifSlug)
	if err != nil {
		writeError(err, w)
		return
	}

	url, err := db.GetURL(id)
	if err != nil {
		writeError(err, w)
		return
	}
	result.URL = url
	result.NSFW = url.NSFW

	verified := mux.Vars(r)["age-verified"] == "yes"
	if verified {
		result.ShowAgeVerification = false
	} else {
		result.ShowAgeVerification = result.NSFW
	}

	err = db.StoreURLView(url)
	if err != nil {
		writeError(err, w)
		return
	}

	err = template.Execute(w, result)
	if err != nil {
		writeError(err, w)
		return
	}
}

func (c *URLController) Index(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	template, err := template.ParseFiles(templates("index.html")...)
	if err != nil {
		writeError(err, w)
		return
	}

	result := IndexResult{}
	count, err := db.GetURLCount()
	result.HumanCount = fmt.Sprintf("%s", humanize.Comma(int64(count)))

	result.SortByTop = mux.Vars(r)["top"] == "top"
	result.SortByShuffle = mux.Vars(r)["shuffle"] == "shuffle"
	result.SortByNew = !result.SortByTop && !result.SortByShuffle
	result.NSFW = mux.Vars(r)["work"] == "nsfw"
	result.Query = r.URL.Query().Get("q")

	verified := mux.Vars(r)["age-verified"] == "yes"
	if verified {
		result.ShowAgeVerification = false
	} else {
		result.ShowAgeVerification = result.NSFW
	}

	if p := r.URL.Query().Get("page"); p != "" {
		currentPage, _ := strconv.Atoi(p)
		result.CurrentPage = currentPage
	} else {
		result.CurrentPage = 1
	}

	q := r.URL.Query()
	q.Set("page", fmt.Sprintf("%v", result.CurrentPage+1))
	result.NextPageLink = "?" + q.Encode()

	if result.SortByTop {
		result.URLs, err = db.GetTopURLs(result.NSFW, result.CurrentPage, PageSize)
	} else if result.SortByShuffle {
		result.URLs, err = db.GetShuffledURLs(result.NSFW, result.CurrentPage, PageSize)
	} else {
		result.URLs, err = db.GetURLs(result.Query, result.NSFW, result.CurrentPage, PageSize)
	}
	if err != nil {
		writeError(err, w)
		return
	}

	err = template.Execute(w, result)
	if err != nil {
		writeError(err, w)
		return
	}
}

func templates(layout string) []string {
	return []string{
		layout,
		"navigation.html",
		"head.html",
		"google-analytics.html",
		"gif-item.html",
		"age-verification.html",
	}
}
