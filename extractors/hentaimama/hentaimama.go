package hentaimama

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/gan-of-culture/get-sauce/request"
	"github.com/gan-of-culture/get-sauce/static"
	"github.com/gan-of-culture/get-sauce/utils"
)

const site = "https://hentaimama.io/"

var reMirrorURLs = regexp.MustCompile(`[^"]*new\d.php\?p=([^"]*)`)
var reExt = regexp.MustCompile(`([a-z][\w]*)(?:\?|$)`)
var reMimeType = regexp.MustCompile(`video/[^']*`)

type extractor struct{}

// New returns a hentaimama extractor.
func New() static.Extractor {
	return &extractor{}
}

func (e *extractor) Extract(URL string) ([]*static.Data, error) {
	URLs := parseURL(URL)
	if len(URLs) == 0 {
		return nil, static.ErrURLParseFailed
	}

	data := []*static.Data{}
	for _, u := range URLs {
		d, err := extractData(u)
		if err != nil {
			return nil, utils.Wrap(err, u)
		}
		data = append(data, d)
	}

	return data, nil
}

func parseURL(URL string) []string {
	if strings.HasPrefix(URL, "https://hentaimama.io/episodes") {
		return []string{URL}
	}

	if !strings.HasPrefix(URL, "https://hentaimama.io/tvshows/") {
		return []string{}
	}

	htmlString, err := request.Get(URL)
	if err != nil {
		return []string{}
	}

	re := regexp.MustCompile(`https://hentaimama.io/episodes[^"]*`)
	return re.FindAllString(htmlString, -1)
}

func extractData(URL string) (*static.Data, error) {
	episodeHtmlString, err := request.Get(URL)
	if err != nil {
		return nil, err
	}

	matchedMirrorURLs := reMirrorURLs.FindAllStringSubmatch(episodeHtmlString, -1)
	if len(matchedMirrorURLs) < 1 {
		return nil, static.ErrDataSourceParseFailed
	}

	idx := -1
	streams := make(map[string]*static.Stream)
	for i, u := range matchedMirrorURLs {
		idx += 1
		b64Path, err := base64.StdEncoding.DecodeString(u[1])
		if err != nil {
			return nil, err
		}
		b64Paths := strings.Split(string(b64Path), "?")

		htmlString, err := request.Get(u[0])
		if err != nil {
			return nil, err
		}

		reSrc := regexp.MustCompile(fmt.Sprintf(`[^"']*/%s[^"']*`, string(b64Paths[0])))
		srcURL := reSrc.FindString(htmlString)

		ext := strings.TrimSuffix(utils.GetLastItemString(reExt.FindStringSubmatch(srcURL)), "?")
		if ext != "m3u8" {
			size, err := request.Size(srcURL, site)
			if err != nil {
				return nil, err
			}

			if ext == "" {
				ext = strings.Split(reMimeType.FindString(srcURL), "/")[1]
			}

			streams[fmt.Sprint(idx)] = &static.Stream{
				URLs: []*static.URL{
					{
						URL: srcURL,
						Ext: ext,
					},
				},
				Size: size,
				Info: fmt.Sprintf("Mirror %d", i+1),
			}
			continue
		}
		idx -= 1

		master, err := request.GetWithHeaders(srcURL, map[string]string{"Referer": srcURL})
		if err != nil {
			return nil, err
		}

		baseURL, err := url.Parse(srcURL)
		if err != nil {
			return nil, err
		}

		streamsTmp, err := utils.ParseM3UMaster(&master)
		if err != nil {
			return nil, err
		}

		for j := len(streamsTmp) - 1; j > -1; j-- {
			idx += 1

			streamTmp := streamsTmp[j]
			mediaURL, err := baseURL.Parse(streamTmp.URLs[0].URL)
			if err != nil {
				return nil, err
			}

			mediaStr, err := request.Get(mediaURL.String())
			if err != nil {
				return nil, err
			}

			URLs, key, err := request.GetM3UMeta(&mediaStr, mediaURL.String())
			if err != nil {
				return nil, err
			}

			streams[fmt.Sprint(idx)] = &static.Stream{
				Type:    static.DataTypeVideo,
				URLs:    URLs,
				Quality: streamTmp.Quality,
				Size:    streamTmp.Size,
				Ext:     "mp4",
				Key:     key,
				Info:    fmt.Sprintf("Mirror %d", i+1),
			}
		}

	}

	return &static.Data{
		Site:    site,
		Title:   utils.GetMeta(&episodeHtmlString, "og:title"),
		Type:    "video",
		Streams: streams,
		URL:     URL,
	}, nil

}
