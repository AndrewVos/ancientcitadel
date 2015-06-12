package minify

import (
	"io/ioutil"
	"net/http"
	"os"
)

var cache = map[string][]byte{}

func getFile(assetPath string) ([]byte, error) {
	if b, ok := cache[assetPath]; ok {
		return b, nil
	}
	file, err := os.Open(assetPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	b, err := ioutil.ReadAll(file)
	return b, err
}

func handler(assetPaths []string, contentType string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		for _, assetPath := range assetPaths {
			b, err := getFile(assetPath)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_, err = w.Write(b)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	})
}

func CSSHandler(assetPaths []string) http.Handler {
	return handler(assetPaths, "text/css")
}

func JSHandler(assetPaths []string) http.Handler {
	return handler(assetPaths, "application/javascript")
}
