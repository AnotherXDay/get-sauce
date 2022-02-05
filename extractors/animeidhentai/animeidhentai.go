package animeidhentai

import (
	"log"
	"net/url"
	"regexp"
	"strings"

	"github.com/gan-of-culture/get-sauce/extractors/htstreaming"
	"github.com/gan-of-culture/get-sauce/extractors/nhplayer"
	"github.com/gan-of-culture/get-sauce/request"
	"github.com/gan-of-culture/get-sauce/static"
	"github.com/gan-of-culture/get-sauce/utils"
)

var reVideoURL = regexp.MustCompile(`https://htstreaming.com/video/([^"]*)`)
var reHTStreamingPlayerURL = regexp.MustCompile(`[^"]*index.php\?data[^"]*`)
var reNHPlayerURL = regexp.MustCompile(`https://nhplayer\.com/v/[^/]+`)

var site string

type extractor struct{}

// New returns a animeidhentai extractor.
func New() static.Extractor {
	return &extractor{}
}

func (e *extractor) Extract(URL string) ([]*static.Data, error) {
	baseURL, err := url.Parse(URL)
	if err != nil {
		return nil, err
	}
	site = baseURL.Host

	URLs := parseURL(URL)
	if len(URLs) == 0 {
		return nil, static.ErrURLParseFailed
	}

	data := []*static.Data{}
	for _, u := range URLs {
		d, err := extractData(u)
		if err != nil {
			if strings.Contains(err.Error(), "video not found") || strings.Contains(err.Error(), "player URL not found") {
				log.Println(utils.Wrap(err, u).Error())
				continue
			}
			return nil, utils.Wrap(err, u)
		}
		data = append(data, d)
	}

	return data, nil
}

func parseURL(URL string) []string {
	if ok, _ := regexp.MatchString(`episode-\d+[/_\-]*`, URL); ok {
		return []string{URL}
	}

	//check if it's an overview/series page maybe
	htmlString, err := request.Get(URL)
	if err != nil {
		return []string{}
	}

	re := regexp.MustCompile(`https[^"\s]*?episode-\d*(?:/*|[-\w]*)"`)
	matchedURLs := re.FindAllString(htmlString, -1)

	out := []string{}
	for _, u := range matchedURLs {
		out = append(out, strings.Trim(u, `"`))
	}

	return utils.RemoveAdjDuplicates(out)
}

func extractData(URL string) (*static.Data, error) {

	htmlString, err := request.Get(URL)
	if err != nil {
		return nil, err
	}

	playerURL := reNHPlayerURL.FindString(htmlString)
	if playerURL != "" {
		data, err := nhplayer.New().Extract(playerURL)
		if err != nil {
			return nil, err
		}
		return data[0], err
	}

	playerURL = reHTStreamingPlayerURL.FindString(htmlString)
	if playerURL == "" {
		return nil, static.ErrURLParseFailed
	}

	return htstreaming.ExtractData(playerURL)
}
