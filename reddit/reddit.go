package reddit

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

type subReddit struct {
	Work string
	Name string
}

func subReddits() []subReddit {
	sfw := []string{
		"gifs", "perfectloops", "creepy_gif", "noisygifs", "analogygifs",
		"reversegif", "funny_gifs", "funnygifs", "aww_gifs", "wheredidthesodago",
		"adventuretimegifs", "animegifs", "communitygifs", "Movie_GIFs", "tvgifs", "gaming_gifs",
		"AnimalsBeingJerks", "AnimalGIFs", "birdreactiongifs", "CatGifs", "catreactiongifs", "slothgifs",
		"Puggifs", "KimJongUnGifs", "SpaceGifs", "physicsgifs", "educationalgifs",
		"chemicalreactiongifs", "mechanical_gifs", "cargifs", "wobblegifs", "SurrealGifs",
	}
	nsfw := []string{
		"gifsgonewild", "porn_gifs", "PornGifs", "NSFW_SEXY_GIF", "nsfwcelebgifs",
		"adultgifs", "NSFW_GIF", "nsfw_gifs", "porngif", "cutegirlgifs", "Hot_Women_Gifs",
		"randomsexygifs", "TittyDrop", "boobbounce", "boobgifs", "celebgifs",
	}
	var subReddits []subReddit
	for _, s := range sfw {
		subReddits = append(subReddits, subReddit{Work: "sfw", Name: s})
	}
	for _, s := range nsfw {
		subReddits = append(subReddits, subReddit{Work: "nsfw", Name: s})
	}
	return subReddits
}

type redditResponse struct {
	Data redditResponseData
}

type redditResponseData struct {
	After    string
	Children []redditResponseChild
}

type redditResponseChild struct {
	Data redditResponseChildData
}

type redditResponseChildData struct {
	Permalink string
	Title     string
	URL       string
}

type RedditURL struct {
	Work      string
	subReddit string
	Title     string
	URL       string
	Permalink string
}

func redditURLs(subReddit subReddit, after string) ([]RedditURL, error) {
	url := fmt.Sprintf("https://api.reddit.com/r/%v/top.json", subReddit.Name)
	if after != "" {
		url += "?after=" + after
	}

	response, err := http.Get(url)
	if err != nil {
		return []RedditURL{}, err
	}
	b, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return []RedditURL{}, err
	}
	var redditResponse redditResponse
	err = json.Unmarshal(b, &redditResponse)
	if err != nil {
		return []RedditURL{}, err
	}

	var urls []RedditURL
	for _, child := range redditResponse.Data.Children {
		url := child.Data.URL
		if strings.Contains(url, "imgur.com") && !strings.HasSuffix(url, ".gif") {
			url = url + ".gif"
		}
		if strings.Contains(url, "gfycat.com") && !strings.HasSuffix(url, ".gif") {
			url = strings.Replace(url, "http://gfycat", "http://giant.gfycat", -1)
			url += ".gif"
		}

		urls = append(urls, RedditURL{
			Work:      subReddit.Work,
			subReddit: subReddit.Name,
			Title:     child.Data.Title,
			URL:       url,
			Permalink: child.Data.Permalink,
		})
	}
	return urls, nil
}

func GetRedditURLs() []RedditURL {
	var groupedRedditURLs [][]RedditURL

	for _, sr := range subReddits() {
		log.Printf("Downloading /r/%v\n", sr.Name)
		urls, err := redditURLs(sr, "")
		if err != nil {
			log.Println(err)
			continue
		}
		groupedRedditURLs = append(groupedRedditURLs, urls)
		log.Println("Sleeping for 2 seconds...")
		time.Sleep(2 * time.Second)
	}

	longestSetOfURLs := 0
	for _, urls := range groupedRedditURLs {
		if len(urls) > longestSetOfURLs {
			longestSetOfURLs = len(urls)
		}
	}
	var redditURLs []RedditURL
	for i := 0; i < longestSetOfURLs; i++ {
		for _, urls := range groupedRedditURLs {
			if i < len(urls) {
				redditURLs = append(redditURLs, urls[i])
			}
		}
	}
	return redditURLs
}
