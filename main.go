package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/golang/glog"
)

var skipUrlPrefixes = []string{"mailto:", "tel:"}

const (
	limitPeriod        = time.Second
	urlsChanBufferSize = 100000
	requestTimeout     = time.Duration(10 * time.Second)
)

func main() {
	var initialURL string
	var maxRequestsPerSecond int
	flag.StringVar(&initialURL, "url", "", "url to parse")
	flag.IntVar(&maxRequestsPerSecond, "n", 1, "maximum amount of requests per second")
	flag.Parse()
	if len(initialURL) == 0 {
		glog.Error("missing parameter: -url")
		os.Exit(1)
	}
	parsedUrl, err := url.Parse(initialURL)
	if err != nil {
		glog.Errorf("couldn't parse url: %+v, err: %+v", initialURL, err)
		os.Exit(1)
	}
	var urlsChan = make(chan string, urlsChanBufferSize)
	var lastRequests []time.Time
	var lastRequestsMtx sync.Mutex

	var alreadyRequestedUrls = make(map[string]struct{})
	var alreadyRequestedUrlsMtx = sync.Mutex{}

	var urlsMap = make(map[string]struct{})
	var urlsMapMtx = sync.Mutex{}

	go func() {
		urlsChan <- initialURL
	}()

	workersAmount := maxRequestsPerSecond * 10
	for i := 0; i < workersAmount; i++ {
		go func() {
			for u := range urlsChan {
				glog.V(4).Infof("urlsChan after reading len: %+v", len(urlsChan))
				lastRequestsMtx.Lock()
				if sleep := func() *time.Duration {
					if maxRequestsPerSecond <= 0 {
						return nil
					}
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
					if len(lastRequests) < maxRequestsPerSecond {
						return nil
					}
					sleep := time.Now().Sub(lastRequests[0])
					go func(u string) {
						urlsChan <- u
					}(u)
					return &sleep
				}(); sleep != nil {
					lastRequestsMtx.Unlock()
					glog.V(4).Infof("sleeping for %s, lastRequests: %+v", sleep, timesStr(lastRequests))
					time.Sleep(*sleep)
					continue
				}
				lastRequests = append(lastRequests, time.Now())
				lastRequestsMtx.Unlock()

				alreadyRequestedUrlsMtx.Lock()
				alreadyRequestedUrls[u] = struct{}{}
				alreadyRequestedUrlsMtx.Unlock()
				urls, err := getUrlsOnThePage(u)
				if err != nil {
					glog.Error(err)
					return
				}
				glog.V(4).Infof("from %s got urls %+v ", u, urls)
				for resultUrl := range urls {
					if !strings.HasPrefix(resultUrl, parsedUrl.Scheme) {
						newUrl := *parsedUrl
						newUrl.Path = resultUrl
						resultUrl = newUrl.String()
					}
					newParsedUrl, err := url.Parse(resultUrl)
					if err != nil {
						glog.Error(err)
						continue
					}
					if newParsedUrl.Host != parsedUrl.Host {
						glog.V(4).Infof("that's url from other domain: %+v, skipping", resultUrl)
						continue
					}
					resultUrl, err = url.QueryUnescape(resultUrl)
					if err != nil {
						glog.Error(err)
						continue
					}
					urlsMapMtx.Lock()
					if _, ok := urlsMap[resultUrl]; ok {
						urlsMapMtx.Unlock()
						continue
					}
					urlsMap[resultUrl] = struct{}{}
					urlsMapMtx.Unlock()
					fmt.Println(resultUrl)
					if alreadyRequested := func() bool {
						alreadyRequestedUrlsMtx.Lock()
						defer alreadyRequestedUrlsMtx.Unlock()
						if _, ok := alreadyRequestedUrls[resultUrl]; ok {
							return true
						}
						return false
					}(); alreadyRequested {
						glog.V(4).Infof("url %+v already requested, skipping ...", resultUrl)
						continue
					}
					go func(foundURL string) {
						urlsChan <- foundURL
						glog.V(4).Infof("urlsChan after writing: len: %+v", len(urlsChan))
					}(resultUrl)
				}
			}
		}()
	}
	forever := make(chan int)
	<-forever
}

func getUrlsOnThePage(url string) (map[string]struct{}, error) {
	glog.Infof("requesting %+v", url)
	client := http.Client{
		Timeout: requestTimeout,
	}
	res, err := client.Get(url)
	if err != nil {
		glog.Errorf("err: %+v\n", err)
		return nil, err
	}
	if res.StatusCode > 399 || res.StatusCode < 200 {
		err := fmt.Errorf("not good status code %+v requesting %+v", res.StatusCode, url)
		glog.Errorf("err: %+v\n", err)
		return nil, err
	}
	defer res.Body.Close()
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		glog.Errorf("err: %+v\n", err)
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
			glog.V(4).Info("skipping ", href)
			return
		}
		resultUrls[href] = struct{}{}
	})
	return resultUrls, nil
}

func skipUrl(u string) bool {
	for _, skip := range skipUrlPrefixes {
		if strings.HasPrefix(u, skip) {
			glog.V(4).Infof("skipping %+v because it has prefix %+v", u, skip)
			return true
		}
	}
	if len(u) == 0 {
		return true
	}
	return false
}

const timePrintFormat = "15:04:05.999"

func timesStr(tt []time.Time) string {
	var results []string
	for _, t := range tt {
		results = append(results, t.Format(timePrintFormat))
	}
	return strings.Join(results, ", ")
}
