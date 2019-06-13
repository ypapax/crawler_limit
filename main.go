package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var skipUrlPrefixes = []string{"mailto:", "tel:"}

const limitPeriod = time.Minute

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	var u string
	var maxRequestsPerMinute int
	flag.StringVar(&u, "url", "", "url to parse")
	flag.IntVar(&maxRequestsPerMinute, "max-reqs-per-minute", 10, "maximum amount of requests per minute")
	flag.Parse()
	if len(u) == 0 {
		log.Printf("missing parameter: -url")
		os.Exit(1)
	}
	parsedUrl, err := url.Parse(u)
	if err != nil {
		log.Printf("couldn't parse url: %+v, err: %+v", u, err)
		os.Exit(1)
	}
	var urlsChan = make(chan string)
	var lastRequests []time.Time
	var lastRequestsMtx sync.Mutex

	go func() {
		urlsChan <- u
	}()

	for u := range urlsChan {
		log.Println("resultUrl", u)
		func() {
			if maxRequestsPerMinute <= 0 {
				return
			}
			lastRequestsMtx.Lock()
			defer lastRequestsMtx.Unlock()
			lastRequests = append(lastRequests, time.Now())
			var obsoleteUpTo int = -1
			for i, lr := range lastRequests {
				if lr.Before(time.Now().Add(-limitPeriod)) {
					obsoleteUpTo = i
				} else {
					break
				}
			}
			if obsoleteUpTo >= 0 {
				lastRequests = lastRequests[obsoleteUpTo+1:]
			}
			if len(lastRequests) <= maxRequestsPerMinute {
				return
			}
			sleep := time.Now().Sub(lastRequests[0])
			log.Println("sleeping for...", sleep)
			time.Sleep(sleep)
		}()
		urls, err := getUrlsOnThePage(u)
		if err != nil {
			log.Printf("err: %+v\n", err)
			continue
		}
		log.Printf("from %s got urls %+v ", u, urls)
		for resultUrl := range urls {
			if !strings.HasPrefix(resultUrl, parsedUrl.Scheme) {
				newUrl := parsedUrl
				newUrl.Path = resultUrl
				resultUrl = newUrl.String()
			}
			go func() {
				urlsChan <- resultUrl
			}()
		}
	}
}

func getUrlsOnThePage(url string) (map[string]struct{}, error) {
	log.Println("requesting", url)
	res, err := http.Get(url)
	if err != nil {
		log.Printf("err: %+v\n", err)
		return nil, err
	}
	if res.StatusCode > 399 || res.StatusCode < 200 {
		err := fmt.Errorf("not good status code %+v requesting %+v", res.StatusCode, url)
		log.Printf("err: %+v\n", err)
		return nil, err
	}
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Printf("err: %+v\n", err)
		return nil, err
	}
	var resultUrls = make(map[string]struct{})
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, ok := s.Attr("href")
		if !ok {
			return
		}
		href = strings.TrimSpace(href)
		if skipUrl(href) {
			log.Println("skipping ", href)
			return
		}
		resultUrls[href] = struct{}{}
	})
	return resultUrls, nil
}

func skipUrl(u string) bool {
	for _, skip := range skipUrlPrefixes {
		if strings.HasPrefix(u, skip) {
			log.Println("skipping ", u)
			return true
		}
	}
	if len(u) == 0 {
		return true
	}
	return false
}
