package minify

import (
	"io"
	"net/http"
	"os"

	"github.com/nytimes/gziphandler"
)

func handler(assetPaths []string, contentType string) http.Handler {
	f := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		for _, assetPath := range assetPaths {
			file, err := os.Open(assetPath)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_, err = io.Copy(w, file)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	})
	return gziphandler.GzipHandler(f)
}

func CSSHandler(assetPaths []string) http.Handler {
	return handler(assetPaths, "text/css")
}

func JSHandler(assetPaths []string) http.Handler {
	return handler(assetPaths, "application/javascript")
}
