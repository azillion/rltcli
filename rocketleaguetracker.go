package main

import (
	"flag"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"regexp"
	"strings"
	"time"
)

var platformFlag = flag.String("p", "all", "platform to search\n\t[all] search all platforms\n\t[steam] search Steam\n\t[xbox] search Xbox One\n\t[ps] search PlayStation 4\n\t")
var playlistFlag = flag.String("q", "3v3", "playlist learderboard to search\n\t[unranked] Un-Ranked\n\t[1v1] Ranked Duel 1v1\n\t[2v2] Ranked Doubles 2v2\n\t[solo] Ranked Solo Standard 3v3\n\t[3v3] Ranked Standard 3v3\n\t")
var pagesFlag = flag.Int("n", 1, "number of pages to search")
var displayLeaderboardFlag = flag.Bool("d", false, "Display Leaderboard")
var regexFlag = flag.String("s", "Squishy", "regex expression to search")
var regexVal *regexp.Regexp
var base_url string = "https://rocketleague.tracker.network/ranked-leaderboards"

func init() {
	flag.Parse()
	regexVal = regexp.MustCompile(*regexFlag)
}

type pageStruct struct {
	page   int
	result string
}

func main() {
	// build url
	fmt.Println(*playlistFlag)
	base_url = fmt.Sprintf("%s/%s/%d?page=", base_url, VerifyPlatform(*platformFlag), PlaylistStringToInt(*playlistFlag))

	start := time.Now()
	ch := make(chan pageStruct, 10)
	urls := make(map[int]string)

	for page := 1; page < *pagesFlag+1; page++ {
		url := fmt.Sprintf("%s%d", base_url, page)
		urls[page-1] = url
		// gocurrency in action
		go fetch(url, page, ch)
	}

	if *displayLeaderboardFlag == true {
		pageResults := make(map[int]string)
		// intentionally blocking
		for range urls {
			result := <-ch
			pageResults[result.page] = result.result
		}
		for i, result := range pageResults {
			fmt.Printf("Page %d\n%s\n\n", i, result)
		}
	} else {
		for range urls {
			result := <-ch
			fmt.Print(result.result)
		}
	}
	fmt.Printf("%.2fs elapsed\n", time.Since(start).Seconds())
}

func fetch(url string, page int, ch chan<- pageStruct) {
	fmt.Println(url)
	doc, err := goquery.NewDocument(url)
	if err != nil {
		ch <- pageStruct{page, fmt.Sprint(err)}
		return
	}
	parsedResults, err := parse(doc)
	if err != nil {
		ch <- pageStruct{-1, fmt.Sprint(err)}
	}
	ch <- pageStruct{page, strings.Join(parsedResults, "")}
	// be a polite internet citizen
	time.Sleep(time.Millisecond * 200)
	return
}

func parse(doc *goquery.Document) ([]string, error) {
	var parsedResults []string
	tableSelection := doc.Find("table")
	if tableSelection.Length() == 0 {
		return nil, fmt.Errorf("No table found")
	}
	tbodySelection := tableSelection.Find("tbody")
	if tbodySelection.Length() == 0 {
		return nil, fmt.Errorf("No tbody found")
	}

	tbodySelection.Find("tr").Slice(1, 102).Each(func(i int, trs *goquery.Selection) {
		var position string
		var username string
		var userURL string
		var rating string
		var gamesAmt string

		if trs.Find("td").Length() >= 1 {
			positionNode := trs.Find("td").Slice(0, 1)
			position = strings.TrimSpace(positionNode.Text())
		}

		if trs.Find("td").Length() >= 2 {
			usernameNode := trs.Find("td").Slice(1, 2)
			username = strings.TrimSpace(usernameNode.Text())
			userURLNode := trs.Find("td").Find("a").Get(1)
			for _, attr := range userURLNode.Attr {
				if attr.Key == "href" {
					userURL = strings.TrimSpace(attr.Val)
				}
			}
		}

		if trs.Find("td").Length() >= 3 {
			ratingNode := trs.Find("td").Slice(2, 3).Find("div .pull-right")
			rating = strings.TrimSpace(ratingNode.Text())
		}

		if trs.Find("td").Length() >= 4 {
			gamesAmtNode := trs.Find("td").Slice(3, 4)
			gamesAmt = strings.TrimSpace(gamesAmtNode.Text())
		}

		if position != "" && username != "" && userURL != "" && rating != "" && gamesAmt != "" {
			if *displayLeaderboardFlag == true || regexVal.MatchString(username) {
				result := fmt.Sprintf("%s | %s | %s | %s | %s\n", position, username, userURL, rating, gamesAmt)
				parsedResults = append(parsedResults, result)
			}
		}
	})
	return parsedResults, nil
}

func PlaylistStringToInt(playlist string) int {
	switch playlist {
	case "unranked":
		return 0
	case "1v1":
		return 10
	case "2v2":
		return 11
	case "solo":
		return 12
	case "3v3":
		return 13
	default:
		return 13
	}
}

func VerifyPlatform(platform string) string {
	platform = strings.ToLower(platform)
	switch platform {
	case "all":
		return "all"
	case "steam":
		return "steam"
	case "xbox":
		return "xbox"
	case "ps":
		return "ps"
	default:
		return "all"
	}
}
