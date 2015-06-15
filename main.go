package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"runtime"

	"github.com/AndrewVos/ancientcitadel/assethandler"
	"github.com/AndrewVos/ancientcitadel/controllers"
	"github.com/AndrewVos/ancientcitadel/db"
	"github.com/AndrewVos/ancientcitadel/ingester"
	"github.com/gorilla/mux"
	"github.com/justinas/alice"
	"github.com/nytimes/gziphandler"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	runtime.GOMAXPROCS(4)
	err := db.Migrate()
	if err != nil {
		log.Fatal(err)
	}

	middleware := alice.New(
		loggingHandler,
		gziphandler.GzipHandler,
		ageVerificationHandler,
	)

	port := flag.String("port", "8080", "the port to bind to")
	flag.Parse()

	r := mux.NewRouter()

	jsHandler := assethandler.JS([]string{
		"assets/scripts/jquery.min.js",
		"assets/scripts/remodal.min.js",
		"assets/scripts/tweet.js",
		"assets/scripts/navigation.js",
		"assets/scripts/play-button.js",
		"assets/scripts/pack.js",
		"assets/scripts/gifs.js",
	})

	cssHandler := assethandler.CSS([]string{
		"assets/styles/bootstrap.min.css",
		"assets/styles/remodal.css",
		"assets/styles/remodal-default-theme.css",
		"assets/styles/main.css",
		"assets/styles/play-button.css",
	})
	handlers := map[string]http.Handler{
		"/compiled.js":            jsHandler,
		"/compiled.css":           cssHandler,
		"/assets/favicons/{icon}": http.StripPrefix("/assets/favicons/", http.FileServer(http.Dir("./assets/favicons/"))),
	}

	for path, handler := range handlers {
		r.Handle(path, middleware.Then(handler))
	}

	urlController := controllers.NewURLController()
	apiController := controllers.NewAPIController()

	handlerFuncs := map[string]func(w http.ResponseWriter, r *http.Request){
		"/api/random/{work:nsfw|sfw}":                  apiController.Random,
		"/api/{work:nsfw|sfw}/{order:new|top|shuffle}": apiController.Index,
		"/":                              urlController.Index,
		"/{top:top}":                     urlController.Index,
		"/{shuffle:shuffle}":             urlController.Index,
		"/{work:nsfw}":                   urlController.Index,
		"/{work:nsfw}/{top:top}":         urlController.Index,
		"/{work:nsfw}/{shuffle:shuffle}": urlController.Index,
		"/gif/{slug}":                    urlController.Show,
		"/tweet/{id:\\d+}":               tweetHandler,
		"/twitter/callback":              twitterCallbackHandler,
		"/sitemap.xml.gz":                sitemapHandler,
	}

	for path, handlerFunc := range handlerFuncs {
		r.Handle(path, middleware.ThenFunc(handlerFunc))
	}

	http.Handle("/", r)
	fmt.Printf("Starting on port %v...\n", *port)

	ingester.Ingest()
	err = http.ListenAndServe("0.0.0.0:"+*port, nil)
	log.Fatal(err)
}
