package reddit

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type SubReddit struct {
	Name           string
	after          string
	finishedPaging bool
}

func (sr *SubReddit) NextPage() ([]RedditURL, error) {
	if sr.finishedPaging {
		return nil, nil
	}

	url := fmt.Sprintf("https://api.reddit.com/r/%v/hot.json", sr.Name)
	if sr.after != "" {
		url += "?after=" + sr.after
	}

	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	b, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	var redditResponse redditResponse
	err = json.Unmarshal(b, &redditResponse)
	if err != nil {
		return nil, err
	}

	var urls []RedditURL
	for _, child := range redditResponse.Data.Children {
		urls = append(urls, RedditURL{
			Title:      child.Data.Title,
			URL:        child.Data.URL,
			Permalink:  child.Data.Permalink,
			CreatedUTC: child.Data.CreatedUTC,
			Over18:     child.Data.Over18,
		})
	}
	sr.after = redditResponse.Data.After
	if sr.after == "" {
		sr.finishedPaging = true
	}

	return urls, nil
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
	Permalink  string
	Title      string
	URL        string
	CreatedUTC float64 `json:"created_utc"`
	Over18     bool    `json:"over_18"`
}

type RedditURL struct {
	Title      string
	URL        string
	Permalink  string
	CreatedUTC float64
	Over18     bool
}
