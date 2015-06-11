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
	GfyName string `json:"gfyName"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	WebMURL string `json:"webmUrl"`
	MP4URL  string `json:"mp4Url"`
}

type UploadedGif struct {
	GfyName  string `json:"gfyname"`
	URLKnown bool   `json:"urlKnown"`
}

func getJSON(url string) ([]byte, error) {
	response, err := http.Get(url)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("couldn't get %v: %v", url, err))
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		return nil, errors.New(fmt.Sprintf("http status %v for %v", response.StatusCode, url))
	}

	b, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("couldn't read %v: %v", url, err))
	}

	type JSONError struct {
		Error string `json:"error"`
	}
	var jsonError JSONError
	json.Unmarshal(b, &jsonError)
	if jsonError.Error != "" {
		return nil, errors.New(
			fmt.Sprintf("error from gfycat %v: %v", url, jsonError.Error),
		)
	}

	return b, nil
}

func gifAlreadyUploaded(gifURL string) (string, error) {
	checkURL := fmt.Sprintf("http://gfycat.com/cajax/checkUrl/%v", url.QueryEscape(gifURL))
	b, err := getJSON(checkURL)
	if err != nil {
		return "", err
	}

	var existing UploadedGif
	err = json.Unmarshal(b, &existing)
	if err != nil {
		return "", err
	}

	if existing.URLKnown {
		return existing.GfyName, nil
	}
	return "", nil
}

func uploadGIF(gifURL string) (string, error) {
	uploadURL := fmt.Sprintf("http://upload.gfycat.com/transcode?fetchUrl=%v", url.QueryEscape(gifURL))
	b, err := getJSON(uploadURL)
	if err != nil {
		return "", err
	}

	var uploadedGif *UploadedGif
	err = json.Unmarshal(b, &uploadedGif)
	if err != nil {
		return "", err
	}

	return uploadedGif.GfyName, nil
}

func testURL(gifURL string) (bool, error) {
	response, err := http.Head(gifURL)
	if err != nil {
		return false, err
	}

	defer response.Body.Close()
	if response.StatusCode == 404 {
		return false, nil
	}
	return true, nil
}

func Gif(gifURL string) (GfyCatInformation, error) {
	test, err := testURL(gifURL)
	if err != nil {
		return GfyCatInformation{}, err
	}
	if !test {
		return GfyCatInformation{}, errors.New(fmt.Sprintf("%q didn't appear healthy", gifURL))
	}

	gfyName, err := gifAlreadyUploaded(gifURL)
	if err != nil {
		return GfyCatInformation{}, err
	}

	if gfyName == "" {
		gfyName, err = uploadGIF(gifURL)
		if err != nil {
			return GfyCatInformation{}, err
		}
	}

	b, err := getJSON(fmt.Sprintf("http://gfycat.com/cajax/get/%v", gfyName))
	if err != nil {
		return GfyCatInformation{}, err
	}

	var j map[string]GfyCatInformation
	err = json.Unmarshal(b, &j)
	if err != nil {
		return GfyCatInformation{}, err
	}

	return j["gfyItem"], nil
}
