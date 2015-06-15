package ingester

import (
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/AndrewVos/ancientcitadel/db"
	"github.com/AndrewVos/ancientcitadel/gifs"
	"github.com/AndrewVos/ancientcitadel/reddit"
)

func Ingest() {
	redditTypes := map[string][]string{
		"sfw": []string{
			"gifs", "perfectloops", "noisygifs", "analogygifs",
			"reversegif", "aww_gifs", "SlyGifs",
			"AnimalsBeingJerks", "shittyreactiongifs", "CatGifs",
			"Puggifs", "SpaceGifs", "physicsgifs", "educationalgifs", "shockwaveporn",
		},
		"nsfw": []string{
			"gifsgonewild", "porn_gifs", "PornGifs", "NSFW_SEXY_GIF",
			"adultgifs", "NSFW_GIF", "nsfw_gifs", "porngif",
		},
	}

	shuffle := func(s []string) {
		for i := range s {
			rand.Seed(time.Now().UnixNano())
			j := rand.Intn(i + 1)
			s[i], s[j] = s[j], s[i]
		}
	}

	shuffle(redditTypes["sfw"])
	shuffle(redditTypes["nsfw"])

	go func() {
		for {
			for _, s := range redditTypes["sfw"] {
				err := updateSubReddit(s, false)
				if err != nil {
					log.Println(err)
				}
			}
			for _, s := range redditTypes["nsfw"] {
				err := updateSubReddit(s, true)
				if err != nil {
					log.Println(err)
				}
			}
		}
	}()
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

func updateSubReddit(name string, nsfw bool) error {
	urlStorer := NewURLStorer()
	defer urlStorer.Wait()

	subReddit := reddit.SubReddit{Name: name}

	fmt.Printf("downloading /r/%v...\n", name)
	for {
		redditURLs, err := subReddit.NextPage()
		if err != nil {
			return err
		}
		if len(redditURLs) == 0 {
			return nil
		}
		for _, redditURL := range redditURLs {
			if validGIFURL(redditURL.URL) == false {
				continue
			}

			if strings.Contains(redditURL.URL, "imgur.com") && !strings.HasSuffix(redditURL.URL, ".gif") {
				redditURL.URL = strings.TrimSuffix(redditURL.URL, ".gifv")
				redditURL.URL = strings.TrimSuffix(redditURL.URL, ".webm")
				redditURL.URL = strings.Replace(redditURL.URL, "http://imgur.com", "http://i.imgur.com", -1)
				redditURL.URL = strings.Replace(redditURL.URL, "/gallery/", "/", -1)
				redditURL.URL = redditURL.URL + ".gif"
			}
			if strings.Contains(redditURL.URL, "gfycat.com") && !strings.HasSuffix(redditURL.URL, ".gif") {
				redditURL.URL = strings.Replace(redditURL.URL, "http://gfycat", "http://giant.gfycat", -1)
				redditURL.URL += ".gif"
			}

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
}

type URLStorer struct {
	urls      chan *db.URL
	waitGroup sync.WaitGroup
}

func (g *URLStorer) Upload(url *db.URL) {
	g.urls <- url
}

func (g *URLStorer) Wait() {
	close(g.urls)
	g.waitGroup.Wait()
}

func NewURLStorer() *URLStorer {
	uploader := &URLStorer{urls: make(chan *db.URL)}

	processors := 5
	for i := 1; i <= processors; i++ {
		uploader.waitGroup.Add(1)
		go func(processor int) {
			host := fmt.Sprintf("http://gifs%v.ancientcitadel.com", processor)

			for url := range uploader.urls {
				result, err := db.GetDownloadResult(url.URL)
				if err != nil {
					log.Println(err)
					continue
				}
				if result != nil && result.Success == false {
					continue
				}

				fmt.Printf("uploading %q to %q...\n", url.URL, host)
				information, err := gifs.Gif(host, url.URL)

				if err != nil {
					log.Println(err)
					err := db.StoreDownloadResult(url.URL, false)
					if err != nil {
						log.Println(err)
					}
					continue
				}
				err = db.StoreDownloadResult(url.URL, true)
				if err != nil {
					log.Println(err)
					continue
				}

				url.WEBMURL = information.WEBMURL
				url.MP4URL = information.MP4URL
				url.ThumbnailURL = information.JPGURL
				url.Width = information.Width
				url.Height = information.Height
				err = db.SaveURL(url)
				if err != nil {
					log.Println(err)
					continue
				}
			}
			uploader.waitGroup.Done()
		}(i)
	}
	return uploader
}
