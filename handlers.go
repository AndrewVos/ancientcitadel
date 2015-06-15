package main

import (
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/AndrewVos/ancientcitadel/db"
	"github.com/ChimeraCoder/anaconda"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/mrjones/oauth"
)

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

func loggingHandler(next http.Handler) http.Handler {
	return handlers.LoggingHandler(os.Stdout, next)
}

func ageVerificationHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer next.ServeHTTP(w, r)

		verified := false
		ageVerified, err := r.Cookie("age-verified")
		if err == nil {
			verified = ageVerified.Value == "yes"
		}

		if r.Method == "POST" {
			r.ParseForm()
			if r.FormValue("age-verified") == "yes" {
				http.SetCookie(w, &http.Cookie{
					Name:    "age-verified",
					Value:   "yes",
					Expires: time.Now().Add(365 * 24 * time.Hour),
					Path:    "/",
				})
				verified = true
			}
		}

		if verified {
			mux.Vars(r)["age-verified"] = "yes"
		}
	})
}
