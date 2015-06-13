package gifs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

type GifInformation struct {
	Error   string `json:"error"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	WEBMURL string `json:"webmurl"`
	MP4URL  string `json:"mp4url"`
	JPGURL  string `json:"jpgurl"`
}

func Gif(host string, gifURL string) (GifInformation, error) {
	uploadURL, err := url.Parse(host + "/upload")
	if err != nil {
		return GifInformation{}, err
	}
	q := uploadURL.Query()
	q.Set("u", gifURL)
	uploadURL.RawQuery = q.Encode()

	response, err := http.Get(uploadURL.String())
	if err != nil {
		return GifInformation{}, err
	}
	defer response.Body.Close()

	b, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return GifInformation{}, err
	}

	if response.StatusCode != 200 {
		return GifInformation{}, errors.New(fmt.Sprintf("http status was %d", response.StatusCode))
	}

	var information GifInformation
	err = json.Unmarshal(b, &information)
	if err != nil {
		return GifInformation{}, err
	}
	if information.Error != "" {
		return GifInformation{}, errors.New(information.Error)
	}

	return information, nil
}
