package reddit

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type SubReddit struct {
	Work string
	Name string
}

func subReddits() []SubReddit {
	sfw := []string{
		"gifs", "perfectloops", "reactiongifs", "creepy_gif", "noisygifs", "analogygifs",
		"reversegif", "funny_gifs", "funnygifs", "aww_gifs", "wheredidthesodago", "shittyreactiongifs",
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
	var subReddits []SubReddit
	for _, s := range sfw {
		subReddits = append(subReddits, SubReddit{Work: "sfw", Name: s})
	}
	for _, s := range nsfw {
		subReddits = append(subReddits, SubReddit{Work: "nsfw", Name: s})
	}
	return subReddits
}

type RedditResponse struct {
	Data RedditResponseData
}

type RedditResponseData struct {
	After    string
	Children []RedditResponseChild
}

type RedditResponseChild struct {
	Data RedditResponseChildData
}

type RedditResponseChildData struct {
	Title string
	URL   string
}

type RedditPage struct {
	SubReddit string
	After     string
	URLs      []RedditURL `json:"urls"`
}

type RedditURL struct {
	Work      string
	SubReddit string
	Title     string
	URL       string
}

func redditURLs(subReddit SubReddit, after string) ([]RedditURL, error) {
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
	var redditResponse RedditResponse
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
			SubReddit: subReddit.Name,
			Title:     child.Data.Title,
			URL:       url,
		})
	}
	return urls, nil
}

var mutex sync.Mutex
var cachedRedditURLs []RedditURL

func SubRedditURLs(work string, page int, pageSize int) []RedditURL {
	var urls []RedditURL
	mutex.Lock()
	for _, url := range cachedRedditURLs {
		if url.Work == work {
			urls = append(urls, url)
		}
	}
	mutex.Unlock()
	startIndex := (page - 1) * pageSize
	var pageOfUrls []RedditURL
	for i := startIndex; i < startIndex+pageSize; i++ {
		if i < len(urls) {
			pageOfUrls = append(pageOfUrls, urls[i])
		}
	}
	return pageOfUrls
}

func UpdateRedditData() {
	var groupedRedditURLs [][]RedditURL

	var mutex sync.Mutex
	var waitGroup sync.WaitGroup
	for _, subReddit := range subReddits() {
		waitGroup.Add(1)
		go func(subReddit SubReddit) {
			defer waitGroup.Done()
			start := time.Now()
			urls, err := redditURLs(subReddit, "")
			if err != nil {
				log.Println(err)
				return
			}
			mutex.Lock()
			groupedRedditURLs = append(groupedRedditURLs, urls)
			mutex.Unlock()

			elapsed := time.Since(start)
			log.Printf("Downloading /r/%v took %s", subReddit.Name, elapsed)
		}(subReddit)
	}
	waitGroup.Wait()

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
	mutex.Lock()
	cachedRedditURLs = redditURLs
	mutex.Unlock()
}
