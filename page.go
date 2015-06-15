package main

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/AndrewVos/ancientcitadel/db"
	"github.com/AndrewVos/ancientcitadel/slug"
	"github.com/dustin/go-humanize"
	"github.com/gorilla/mux"
)

const (
	PageSize = 20
)

type Page struct {
	r    *http.Request
	url  *db.URL
	urls *[]db.URL
}

func NewPageFromRequest(w http.ResponseWriter, r *http.Request) (*Page, error) {
	page := &Page{r: r}
	return page, nil
}

func (p *Page) currentPage() int {
	if p := p.r.URL.Query().Get("page"); p != "" {
		currentPage, _ := strconv.Atoi(p)
		return currentPage
	}
	return 1
}

func (p *Page) NextPage() string {
	q := p.r.URL.Query()
	q.Set("page", fmt.Sprintf("%v", p.currentPage()+1))
	return "?" + q.Encode()
}

func (p *Page) URLCount() (string, error) {
	count, err := db.GetURLCount()
	return fmt.Sprintf("%s", humanize.Comma(int64(count))), err
}

func (p *Page) Query() string {
	return p.r.URL.Query().Get("q")
}

func (p *Page) SortByNew() bool {
	return !p.SortByTop() && !p.SortByShuffle()
}

func (p *Page) SortByTop() bool {
	return mux.Vars(p.r)["top"] == "top"
}

func (p *Page) SortByShuffle() bool {
	return mux.Vars(p.r)["shuffle"] == "shuffle"
}

func (p *Page) NSFW() bool {
	return mux.Vars(p.r)["work"] == "nsfw"
}

func (p *Page) ShowAgeVerification() (bool, error) {
	verified := mux.Vars(p.r)["age-verified"] == "yes"
	if verified {
		return false, nil
	}

	url, err := p.URL()
	if err != nil {
		return false, err
	}
	if url != nil {
		return url.NSFW, nil
	}

	return p.NSFW(), nil
}

func (p *Page) URL() (*db.URL, error) {
	if p.url != nil {
		return p.url, nil
	}

	gifSlug := mux.Vars(p.r)["slug"]
	if gifSlug == "" {
		return nil, nil
	}
	id, err := slug.Parse(gifSlug)

	url, err := db.GetURL(id)
	if err != nil {
		return nil, err
	}
	p.url = url
	err = db.StoreURLView(url)
	return url, err
}

func (p *Page) URLs() (*[]db.URL, error) {
	if p.urls != nil {
		return p.urls, nil
	}

	var urls []db.URL
	var err error

	if p.SortByTop() {
		urls, err = db.GetTopURLs(p.NSFW(), p.currentPage(), PageSize)
	} else if p.SortByShuffle() {
		urls, err = db.GetShuffledURLs(p.NSFW(), p.currentPage(), PageSize)
	} else {
		urls, err = db.GetURLs(p.Query(), p.NSFW(), p.currentPage(), PageSize)
	}
	p.urls = &urls
	return p.urls, err
}
