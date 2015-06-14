package assethandler

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
)

var mutex = &sync.Mutex{}
var cache map[string]*bytes.Buffer

func init() {
	cache = map[string]*bytes.Buffer{}
}

func JS(assets []string) http.Handler {
	return handler(assets, "application/javascript")
}

func CSS(assets []string) http.Handler {
	return handler(assets, "text/css")
}

func handler(assets []string, contentType string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)

		buffer, err := retrieveAssets(assets)
		_, err = w.Write(buffer.Bytes())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}

func retrieveAssets(assets []string) (*bytes.Buffer, error) {
	key := assetsKey(assets)
	buffer, ok := cache[key]

	if ok {
		return buffer, nil
	}

	mutex.Lock()
	defer mutex.Unlock()

	buffer = &bytes.Buffer{}
	for _, asset := range assets {
		file, err := os.Open(asset)
		if err != nil {
			return nil, err
		}
		defer file.Close()

		_, err = io.Copy(buffer, file)
		if err != nil {
			return nil, err
		}
	}
	cache[key] = buffer
	return buffer, nil
}

func assetsKey(assets []string) string {
	return strings.Join(assets, ",")
}
