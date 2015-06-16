package ingester

import (
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/AndrewVos/ancientcitadel/db"
	"github.com/AndrewVos/ancientcitadel/reddit"
)

var sfw = []string{
	"gifs", "perfectloops", "noisygifs", "analogygifs",
	"reversegif", "aww_gifs", "SlyGifs",
	"AnimalsBeingJerks", "shittyreactiongifs", "CatGifs",
	"Puggifs", "SpaceGifs", "physicsgifs", "educationalgifs", "shockwaveporn",
}

var nsfw = []string{"gifsgonewild", "porn_gifs", "PornGifs", "NSFW_SEXY_GIF",
	"adultgifs", "NSFW_GIF", "nsfw_gifs", "porngif",
}

func Ingest() {
	reddits := map[string]bool{}

	shuffleStrings(sfw)
	for _, name := range sfw {
		reddits[name] = false
	}

	shuffleStrings(nsfw)
	for _, name := range nsfw {
		reddits[name] = true
	}

	go func() {
		for name, nsfw := range reddits {
			err := updateSubReddit(name, nsfw)
			if err != nil {
				log.Println(err)
			}
		}
	}()
}

func updateSubReddit(name string, nsfw bool) error {
	urlStorer := NewURLStorer()
	defer urlStorer.Wait()

	subReddit := reddit.SubReddit{Name: name}

	fmt.Printf("downloading (nsfw=%v) /r/%v...\n", nsfw, name)
	for pr := range subReddit.AllPages() {
		if pr.Error != nil {
			return pr.Error
		}
		for _, redditURL := range pr.RedditURLs {
			if validGIFURL(redditURL.URL) == false {
				continue
			}
			redditURL.URL = makeValidGIFURL(redditURL.URL)
			sourceURL := "https://reddit.com" + redditURL.Permalink

			url := db.URL{
				Title:     redditURL.Title,
				NSFW:      redditURL.Over18,
				SourceURL: sourceURL,
				URL:       redditURL.URL,
				CreatedAt: time.Unix(int64(redditURL.CreatedUTC), 0),
			}

			if nsfw != url.NSFW {
				continue
			}

			id, err := db.ExistsInDB(url)
			if err != nil {
				log.Println(err)
				continue
			}

			if id != 0 {
				db.UpdateURL(id, url)
				continue
			}

			urlStorer.Upload(&url)
		}
	}
	return nil
}

func validGIFURL(url string) bool {
	if strings.HasSuffix(url, ".jpg") {
		return false
	}
	if strings.Contains(url, "youtube.com") {
		return false
	}

	if strings.Contains(url, "imgur.com") {
		return true
	}
	if strings.Contains(url, "gfycat.com") {
		return true
	}
	if strings.HasSuffix(url, ".gif") {
		return true
	}
	return false
}

func makeValidGIFURL(url string) string {
	if strings.Contains(url, "imgur.com") && !strings.HasSuffix(url, ".gif") {
		url = strings.TrimSuffix(url, ".gifv")
		url = strings.TrimSuffix(url, ".webm")
		url = strings.Replace(url, "http://imgur.com", "http://i.imgur.com", -1)
		url = strings.Replace(url, "/gallery/", "/", -1)
		url = url + ".gif"
	}
	if strings.Contains(url, "gfycat.com") && !strings.HasSuffix(url, ".gif") {
		url = strings.Replace(url, "http://gfycat", "http://giant.gfycat", -1)
		url += ".gif"
	}
	return url
}

func shuffleStrings(s []string) []string {
	for i := range s {
		rand.Seed(time.Now().UnixNano())
		j := rand.Intn(i + 1)
		s[i], s[j] = s[j], s[i]
	}
	return s
}
