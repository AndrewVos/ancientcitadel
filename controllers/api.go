package controllers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/AndrewVos/ancientcitadel/db"
	"github.com/gorilla/mux"
)

type APIController struct{}

type JSONError struct {
	Error string `json:"error"`
}

func NewAPIController() *APIController {
	return &APIController{}
}

func writeJSONError(w http.ResponseWriter, err error) {
	b, _ := json.Marshal(JSONError{Error: err.Error()})
	w.Write(b)
	log.Println(err)
	return
}

func (c *APIController) Docs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	err := templates.ExecuteTemplate(w, "api", nil)
	if err != nil {
		writeError(err, w)
	}
}

func (c *APIController) Index(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	nsfw := mux.Vars(r)["work"] == "nsfw"
	order := mux.Vars(r)["order"]
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		page, _ = strconv.Atoi(p)
	}
	query := r.URL.Query().Get("q")

	var err error
	urls := []db.URL{}

	if order == "new" || order == "" {
		urls, err = db.GetURLs(query, nsfw, page, PageSize)
	} else if order == "top" {
		urls, err = db.GetTopURLs(nsfw, page, PageSize)
	} else if order == "shuffle" {
		urls, err = db.GetShuffledURLs(nsfw, page, PageSize)
	}
	if err != nil {
		writeJSONError(w, err)
		return
	}

	b, err := json.Marshal(urls)
	if err != nil {
		writeJSONError(w, err)
		return
	}

	_, err = w.Write(b)
	if err != nil {
		writeJSONError(w, err)
		return
	}
}

func (c *APIController) Random(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	nsfw := mux.Vars(r)["work"] == "nsfw"

	url, err := db.GetRandomURL(nsfw)
	if err != nil {
		writeJSONError(w, err)
		return
	}
	b, err := json.MarshalIndent(url, " ", "")
	if err != nil {
		writeJSONError(w, err)
		return
	}
	w.Write(b)
}
