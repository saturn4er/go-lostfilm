package lostfilm

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"regexp"

	"errors"

	"github.com/PuerkitoBio/goquery"
)

type EpisodeLink struct {
	Format      string
	Quality     string
	Size        string
	TorrentLink string
}
type Episode struct {
	EpisodeNumber string
	SerieID       string
	SeasonNumber  string
	Title         string
	EngTitle      string
	Available     bool
}

type Season struct {
	N        int
	Episodes []Episode
}

type Serie struct {
	Title     string `json:"title"`
	TitleOrig string `json:"title_orig"`
	Alias     string `json:"alias"`
	Year      int    `json:"date,string"`
	Genres    string `json:"genres"`
	ID        string `json:"id"`
}

type serialsResponse struct {
	Data   []Serie `json:"data"`
	Result string  `json:"result"`
}

func (l *Lostfilm) GetAllSeries() ([]Serie, error) {
	var result []Serie
	params := url.Values{
		"act":  {"serial"},
		"type": {"search"},
		"t":    {"0"},
		"s":    {"3"},
	}
	var offset int64
	for {
		resp, err := l.request("POST", "https://www.lostfilm.tv/ajaxik.php?"+params.Encode())
		if err != nil {
			return nil, err
		}
		p, err := l.decodeResponse(resp)
		resp.Body.Close()

		d := serialsResponse{}
		err = json.Unmarshal([]byte(p), &d)
		if err != nil {
			return nil, err
		}
		if len(d.Data) == 0 {
			break
		}
		result = append(result, d.Data...)
		offset += 10
		params.Set("o", strconv.FormatInt(offset, 10))
	}
	return result, nil
}

func (l *Lostfilm) GetSerieSeasons(s *Serie) ([]Season, error) {
	var result []Season
	resp, err := l.request("POST", "https://www.lostfilm.tv/series/"+s.Alias+"/seasons")
	if err != nil {
		return nil, err
	}
	p, err := l.decodeResponse(resp)
	resp.Body.Close()
	d, err := goquery.NewDocumentFromReader(strings.NewReader(p))
	if err != nil {
		return nil, err
	}
	serieBlocks := d.Find(".serie-block")

	serieBlocks.Each(func(_ int, serieBlock *goquery.Selection) {
		d, ok := serieBlock.Find("table").Attr("id")
		if !ok {
			fmt.Println("Can't find season table")
			return
		}
		if !strings.HasPrefix(d, "season_series") {
			fmt.Println("Bad table id field: " + d)
			return
		}
		sSeasonN := strings.TrimPrefix(d, "season_series_")
		seasonN, err := strconv.ParseInt(sSeasonN, 10, 64)
		if err != nil {
			fmt.Printf("Can't parse series season number(%v): %v", sSeasonN, err)
			return
		}
		episodes, err := getSeasonEpisodes(s.ID, serieBlock)
		if err != nil {
			fmt.Println("Cant' get season episodes: ", err)
			return
		}
		season := Season{
			N:        int(seasonN),
			Episodes: episodes,
		}
		result = append(result, season)
	})
	return result, nil

}

var episodeLinksRegex = regexp.MustCompile("location.replace\\(\"(http.//retre.org.+?)\"\\)")
var linkDescriptionRegex = regexp.MustCompile("^Видео: (.+?). Размер: (.+?). Перевод: (.+?)$")
func (l *Lostfilm) GetEpisodeLinks(e *Episode) ([]EpisodeLink, error) {
	if e.Available == false {
		return nil, errors.New("can't get links of unavailable episode")
	}
	resp, _ := l.request("POST", "https://www.lostfilm.tv/v_search.php?c="+e.SerieID+"&s="+e.SeasonNumber+"&e="+e.EpisodeNumber)
	p, _ := l.decodeResponse(resp)
	resp.Body.Close()
	retreLinkM := episodeLinksRegex.FindAllStringSubmatch(p, -1)
	if len(retreLinkM) == 0 {
		fmt.Println("Can't get ReTre link. Can't parse it from location")
		return nil, errors.New(p)
	}
	retreLink := retreLinkM[0][1]
	resp, err := l.request("GET", retreLink)
	if err != nil {
		return nil, err
	}
	p, err = l.decodeResponse(resp)
	if err != nil {
		return nil, err
	}
	d, err := goquery.NewDocumentFromReader(strings.NewReader(p))
	if err != nil {
		return nil, err
	}
	var result []EpisodeLink
	d.Find(".inner-box--list").Find(".inner-box--item").Each(func(_ int, s *goquery.Selection) {
		l := EpisodeLink{}
		l.Format = strings.TrimSpace(s.Find(".inner-box--label").Text())
		link, ok := s.Find(".main a").Attr("href")
		if !ok {
			fmt.Println("Can't find link href attribute")
			return
		}
		descr := s.Find(".inner-box--desc").Text()
		descrM := linkDescriptionRegex.FindAllStringSubmatch(descr, -1)
		if len(descrM)==0 {
			fmt.Println("Can't parse link description: "+descr)
			return
		}
		l.Quality = descrM[0][1]
		l.Size = descrM[0][2]
		l.TorrentLink = link
		result = append(result, l)

	})
	return result, nil
}

var episodeNumberRegex = regexp.MustCompile("^([0-9]+?) сезон ([0-9]+?) серия$")
var episodeNameRegexFuture = regexp.MustCompile("(?s:^(.+?)<br/><span class=\"small-text\">(.+?)</span>$)")
var episodeNameRegex = regexp.MustCompile("(?s:^<div>(.+?)<br/>.+?<span class=\"gray-color2 small-text\">(.+?)</span>.+?</div>$)")

func getSeasonEpisodes(serieID string, selection *goquery.Selection) ([]Episode, error) {
	var result []Episode
	selection.Find("table tr").Each(func(_ int, s *goquery.Selection) {
		e := Episode{
			SerieID: serieID,
		}
		e.Available = !s.HasClass("not-available")

		epNs := s.Find(".beta").Text()
		epN := episodeNumberRegex.FindAllStringSubmatch(epNs, -1)
		if len(epN) == 0 {
			fmt.Println("Can't parse episode number: " + epNs)
			return
		}
		e.SeasonNumber = epN[0][1]
		e.EpisodeNumber = epN[0][2]

		epNamesS, err := s.Find(".gamma").Html()
		if err != nil {
			fmt.Println("Can't get html of episode names: ", err)
			return
		}
		epNamesS = strings.TrimSpace(epNamesS)
		if e.Available {
			epNames := episodeNameRegex.FindAllStringSubmatch(epNamesS, -1)[0][1:]
			e.Title = strings.TrimSpace(epNames[0])
			e.EngTitle = strings.TrimSpace(epNames[1])
		} else {
			epNames := episodeNameRegexFuture.FindAllStringSubmatch(epNamesS, -1)[0][1:]
			e.Title = strings.TrimSpace(epNames[0])
			e.EngTitle = strings.TrimSpace(epNames[1])
		}
		result = append(result, e)

	})
	return result, nil
}
