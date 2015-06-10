package gfycat

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

type GfyCatInformation struct {
	GfyName string
	WebMURL string
	MP4URL  string
	Width   int
	Height  int
}

func Gif(gifURL string) (*GfyCatInformation, error) {
	type UploadedGif struct {
		Error   string `json:"error"`
		GfyName string `json:"gfyname"`
		WebMURL string `json:"webmUrl"`
		MP4URL  string `json:"mp4Url"`
	}
	type GfyItem struct {
		Width  int
		Height int
	}

	uploadURL := fmt.Sprintf("http://upload.gfycat.com/transcode?fetchUrl=%v", url.QueryEscape(gifURL))
	uploadResponse, err := http.Get(uploadURL)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("couldn't upload gif to gfycat, but got this?\n%v", err))
	}
	defer uploadResponse.Body.Close()

	if uploadResponse.StatusCode != 200 {
		return nil, errors.New(fmt.Sprintf("gfycat didn't give us a 200, but got this?\n%v", err))
	}

	b, err := ioutil.ReadAll(uploadResponse.Body)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Couldn't read body from %v", uploadURL))
	}
	var uploadedGif UploadedGif
	err = json.Unmarshal(b, &uploadedGif)

	if err != nil {
		return nil, errors.New(
			fmt.Sprintf("couldn't decode this from gfycat:\n%v\nError:\n%v\nURL: %v", string(b), err, uploadURL),
		)
	}
	if uploadedGif.Error != "" {
		return nil, errors.New(fmt.Sprintf("got error from url %q\n%v", uploadURL, uploadedGif.Error))
	}

	getURL := fmt.Sprintf("http://gfycat.com/cajax/get/%v", uploadedGif.GfyName)
	getResponse, err := http.Get(getURL)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("couldn't get gif from gfycat, but got this?\n%v\n", err))
	}

	if getResponse.StatusCode != 200 {
		return nil, errors.New(
			fmt.Sprintf(
				"couldn't get extra information from gfycat about %v, status code %v",
				uploadedGif.GfyName,
				getResponse.StatusCode,
			))
	}

	b, err = ioutil.ReadAll(getResponse.Body)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Couldn't read body from %v", getURL))
	}
	defer getResponse.Body.Close()

	var j map[string]GfyItem
	err = json.Unmarshal(b, &j)
	if err != nil {
		return nil, errors.New(
			fmt.Sprintf("couldn't decode this from gfycat:\n%v\nError:\n%v\nURL: %v", string(b), err, getURL),
		)
	}

	return &GfyCatInformation{
		GfyName: uploadedGif.GfyName,
		WebMURL: uploadedGif.WebMURL,
		MP4URL:  uploadedGif.MP4URL,
		Width:   j["gfyItem"].Width,
		Height:  j["gfyItem"].Height,
	}, nil
}
