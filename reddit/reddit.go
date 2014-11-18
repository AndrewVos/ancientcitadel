package reddit

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"
)

func subReddits() map[string][]string {
	return map[string][]string{
		"sfw": []string{
			"gifs", "perfectloops",
		},
		"nsfw": []string{
			"gifsgonewild",
		},
	}
	// return map[string][]string{
	// 	"sfw": []string{
	// 		"gifs", "perfectloops", "reactiongifs", "creepy_gif", "noisygifs", "analogygifs",
	// 		"reversegif", "funny_gifs", "funnygifs", "aww_gifs", "wheredidthesodago", "shittyreactiongifs",
	// 		"adventuretimegifs", "animegifs", "communitygifs", "Movie_GIFs", "tvgifs", "gaming_gifs",
	// 		"AnimalsBeingJerks", "AnimalGIFs", "birdreactiongifs", "CatGifs", "catreactiongifs", "slothgifs",
	// 		"Puggifs", "celebgifs", "KimJongUnGifs", "SpaceGifs", "physicsgifs", "educationalgifs",
	// 		"chemicalreactiongifs", "mechanical_gifs", "cargifs", "wobblegifs", "SurrealGifs",
	// 	},
	// 	"nsfw": []string{
	// 		"gifsgonewild", "porn_gifs", "PornGifs", "NSFW_SEXY_GIF", "nsfwcelebgifs",
	// 		"adultgifs", "NSFW_GIF", "nsfw_gifs", "porngif", "cutegirlgifs", "Hot_Women_Gifs",
	// 		"randomsexygifs", "TittyDrop", "boobbounce", "boobgifs",
	// 	},
	// }
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
	SubReddit string
	Title     string
	URL       string
}

func redditPage(subReddit string, after string) (RedditPage, error) {
	url := fmt.Sprintf("https://api.reddit.com/r/%v/hot.json", subReddit)
	if after != "" {
		url += "?after=" + after
	}

	response, err := http.Get(url)
	if err != nil {
		return RedditPage{}, err
	}
	b, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return RedditPage{}, err
	}
	var redditResponse RedditResponse
	err = json.Unmarshal(b, &redditResponse)
	if err != nil {
		return RedditPage{}, err
	}

	page := RedditPage{}
	page.SubReddit = subReddit
	page.After = redditResponse.Data.After
	for _, child := range redditResponse.Data.Children {
		url := child.Data.URL
		if strings.Contains(url, "imgur.com") && !strings.HasSuffix(url, ".gif") {
			url = url + ".gif"
		}
		if strings.Contains(url, "gfycat.com") && !strings.HasSuffix(url, ".gif") {
			url = strings.Replace(url, "http://gfycat", "http://giant.gfycat", -1)
			url += ".gif"
		}

		page.URLs = append(page.URLs, RedditURL{
			SubReddit: subReddit,
			Title:     child.Data.Title,
			URL:       url,
		})
	}
	return page, nil
}

var mutex sync.Mutex
var cachedRedditURLs map[string][]RedditURL

func SubRedditURLs(work string) []RedditURL {
	mutex.Lock()
	urls := cachedRedditURLs[work]
	mutex.Unlock()
	return urls
}

func init() {
	updateRedditData()
}

func updateRedditData() {
	redditPages := map[string][]RedditPage{}
	redditURLs := map[string][]RedditURL{}

	for _, work := range []string{"sfw", "nsfw"} {
		subReddits := subReddits()[work]
		for _, subReddit := range subReddits {
			fmt.Printf("Getting /r/%v\n", subReddit)
			page, err := redditPage(subReddit, "")
			if err != nil {
				log.Println(err)
				continue
			}
			redditPages[work] = append(redditPages[work], page)
		}

		longestSetOfURLs := 0
		for _, redditPage := range redditPages[work] {
			if len(redditPage.URLs) > longestSetOfURLs {
				longestSetOfURLs = len(redditPage.URLs)
			}
		}
		for i := 0; i < longestSetOfURLs; i++ {
			for _, redditPage := range redditPages[work] {
				if i < len(redditPage.URLs) {
					redditURLs[work] = append(redditURLs[work], redditPage.URLs[i])
				}
			}
		}
	}
	mutex.Lock()
	cachedRedditURLs = redditURLs
	mutex.Unlock()
}
