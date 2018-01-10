package main

import (
	"flag"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/ryanuber/columnize"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var platformFlag string
var playlistFlag string
var pagesFlag int
var displayLeaderboardFlag bool
var regexFlag string
var regexVal *regexp.Regexp
var replacer = strings.NewReplacer("|", "", "\"", "", ",", "")
var base_url string = "https://rocketleague.tracker.network/ranked-leaderboards"

func init() {
	flag.StringVar(&platformFlag, "p", "all", "platform to search\n\t[all] search all platforms\n\t[steam] search Steam\n\t[xbox] search Xbox One\n\t[ps] search PlayStation 4\n\t")
	flag.StringVar(&playlistFlag, "q", "3v3", "playlist learderboard to search\n\t[unranked] Un-Ranked\n\t[1v1] Ranked Duel 1v1\n\t[2v2] Ranked Doubles 2v2\n\t[solo] Ranked Solo Standard 3v3\n\t[3v3] Ranked Standard 3v3\n\t")
	flag.IntVar(&pagesFlag, "n", 1, "number of pages to search")
	flag.BoolVar(&displayLeaderboardFlag, "d", false, "Display Leaderboard")
	flag.StringVar(&regexFlag, "s", "Squishy", "regex expression to search")
	flag.Parse()
	regexFlag = replacer.Replace(regexFlag)
	regexVal = regexp.MustCompile(regexFlag)
}

type LeaderboardPage struct {
	page int
	rows []LeaderboardRow
	err  error
}

func (page LeaderboardPage) ToString() string {
	if len(page.rows) == 0 {
		return ""
	}

	var rowStrings []string
	for _, row := range page.rows {
		if row.player != "" && row.position != 0 {
			rowStrings = append(rowStrings, row.ToString())
		}
	}
	rowStrings = append(rowStrings, "")
	return columnize.SimpleFormat(rowStrings)
}

type LeaderboardRow struct {
	position     int
	player       string
	playerURL    string
	rating       int
	gamesAmt     int
	pagePosition int
	err          error
}

func (row LeaderboardRow) ToString() string {
	return fmt.Sprintf("%d | %s | %s | %d | %d", row.position, row.player, row.playerURL, row.rating, row.gamesAmt)
}

func (row LeaderboardRow) Load(position string, player string, playerURL string, rating string, gamesAmt string, pagePosition int) LeaderboardRow {
	posint, err := strconv.Atoi(position)
	if err != nil {
		fmt.Println(err)
	}
	ratingint, err := strconv.Atoi(rating)
	if err != nil {
		fmt.Println(err)
	}
	gamesAmtint, err := strconv.Atoi(gamesAmt)
	if err != nil {
		fmt.Println(err)
	}
	return LeaderboardRow{
		posint,
		player,
		playerURL,
		ratingint,
		gamesAmtint,
		pagePosition,
		nil,
	}
}

func main() {
	// build url
	base_url = fmt.Sprintf("%s/%s/%d?page=", base_url, VerifyPlatform(platformFlag), PlaylistStringToInt(playlistFlag))

	start := time.Now()
	ch := make(chan LeaderboardPage, 10)
	urls := make([]string, pagesFlag)

	for page := 1; page < pagesFlag+1; page++ {
		url := fmt.Sprintf("%s%d", base_url, page)
		urls[page-1] = url
		// gocurrency in action
		go fetch(url, page, ch)

		// be a polite internet citizen
		time.Sleep(time.Millisecond * 200)
	}

	if displayLeaderboardFlag == true {
		pageResults := make([]string, pagesFlag)
		// intentionally blocking
		for range urls {
			result := <-ch
			if result.err != nil {
				fmt.Println(result.err)
			} else {
				pageResults[result.page-1] = result.ToString()
			}
		}
		if len(pageResults) > 0 {
			for i, result := range pageResults {
				fmt.Printf("%s\n\n", i+1, result)
			}
		}
	} else {
		for range urls {
			result := <-ch
			if result.err != nil {
				fmt.Println(result.err)
			} else {
				fmt.Print(result.ToString())
			}
		}
	}
	fmt.Printf("%.2fs elapsed\n", time.Since(start).Seconds())
}

func fetch(url string, page int, ch chan<- LeaderboardPage) {
	fmt.Printf("Page %d\nURL  %s\n", page, url)
	chr := make(chan LeaderboardRow, 10)

	doc, err := goquery.NewDocument(url)
	if err != nil {
		ch <- LeaderboardPage{0, nil, err}
		return
	}

	tableSelection := doc.Find("table")
	if tableSelection.Length() == 0 {
		ch <- LeaderboardPage{0, nil, fmt.Errorf("No table found")}
		return
	}
	tbodySelection := tableSelection.Find("tbody")
	if tbodySelection.Length() == 0 {
		ch <- LeaderboardPage{0, nil, fmt.Errorf("No tbody found")}
		return
	}

	trSelection := tbodySelection.Find("tr")
	if trSelection.Length() <= 1 {
		ch <- LeaderboardPage{0, nil, fmt.Errorf("No tr found")}
		return
	}

	// Slice rows
	trSelection = trSelection.Slice(1, trSelection.Length())

	var count int
	trSelection.Each(func(i int, trs *goquery.Selection) {
		go parseRow(i, trs, chr)
		count++
	})

	leaderboardPage := make([]LeaderboardRow, count)
	for range leaderboardPage {
		leaderboardRow := <-chr
		// if leaderboardRow.err != nil {
		// 	fmt.Println(leaderboardRow.err)
		// }
		leaderboardPage[leaderboardRow.pagePosition] = leaderboardRow
	}

	ch <- LeaderboardPage{page, leaderboardPage, nil}
	return
}

func parseRow(i int, trs *goquery.Selection, ch chan<- LeaderboardRow) {
	var position string
	var username string
	var userURL string
	var rating string
	var gamesAmt string

	if trs.Find("td").Length() >= 1 {
		positionNode := trs.Find("td").Slice(0, 1)
		position = strings.TrimSpace(positionNode.Text())
		position = replacer.Replace(position)
	}

	if trs.Find("td").Length() >= 2 {
		usernameNode := trs.Find("td").Slice(1, 2)
		username = strings.TrimSpace(usernameNode.Text())
		username = replacer.Replace(username)
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
		rating = replacer.Replace(rating)
	}

	if trs.Find("td").Length() >= 4 {
		gamesAmtNode := trs.Find("td").Slice(3, 4)
		gamesAmt = strings.TrimSpace(gamesAmtNode.Text())
		gamesAmt = replacer.Replace(gamesAmt)
	}

	if position != "" && username != "" && userURL != "" && rating != "" && gamesAmt != "" {
		if displayLeaderboardFlag == true || regexVal.MatchString(username) {
			ch <- new(LeaderboardRow).Load(position, username, userURL, rating, gamesAmt, i)
			return
		}
	}
	ch <- LeaderboardRow{0, "", "", 0, 0, i, fmt.Errorf("No row found\n%s | %s | %s | %s | %s", position, username, userURL, rating, gamesAmt)}
	return
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
