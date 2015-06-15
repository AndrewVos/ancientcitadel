package ingester

import (
	"fmt"
	"log"
	"sync"

	"github.com/AndrewVos/ancientcitadel/db"
	"github.com/AndrewVos/ancientcitadel/gifs"
)

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

func storeURL(host string, url *db.URL) error {
	result, err := db.GetDownloadResult(url.URL)
	if err != nil {
		return err
	}
	if result != nil && result.Success == false {
		return nil
	}

	fmt.Printf("uploading %q to %q...\n", url.URL, host)
	information, err := gifs.Gif(host, url.URL)
	if e := db.StoreDownloadResult(url.URL, err == nil); e != nil {
		log.Printf("couldn't store download result because: %v\n", e)
	}
	if err != nil {
		return err
	}

	url.WEBMURL = information.WEBMURL
	url.MP4URL = information.MP4URL
	url.ThumbnailURL = information.JPGURL
	url.Width = information.Width
	url.Height = information.Height
	return db.SaveURL(url)
}

func NewURLStorer() *URLStorer {
	uploader := &URLStorer{urls: make(chan *db.URL)}

	processors := 5
	for i := 1; i <= processors; i++ {
		uploader.waitGroup.Add(1)
		go func(processor int) {
			host := fmt.Sprintf("http://gifs%v.ancientcitadel.com", processor)
			for url := range uploader.urls {
				err := storeURL(host, url)
				if err != nil {
					log.Println(err)
				}
			}
			uploader.waitGroup.Done()
		}(i)
	}
	return uploader
}
