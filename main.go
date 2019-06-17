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

	maxRequestsToWorkersMultiplier = 10
)

func main() {
	var initialURL string
	var maxRequestsPerSecond int
	flag.StringVar(&initialURL, "url", "", "url to crawl on")
	flag.IntVar(&maxRequestsPerSecond, "n", 1, "maximum amount of requests per second")
	flag.Parse()
	if len(initialURL) == 0 {
		glog.Error("missing parameter: -url")
		glog.Flush()
		os.Exit(1)
	}
	if maxRequestsPerSecond <= 0 {
		glog.Error("-n parameter should be positive")
		glog.Flush()
		os.Exit(1)
	}
	parsedUrl, err := url.Parse(initialURL)
	if err != nil {
		glog.Errorf("couldn't parse url: %+v, err: %+v", initialURL, err)
		glog.Flush()
		os.Exit(1)
	}
	var urlsChan = make(chan string, urlsChanBufferSize)
	var lastRequests []time.Time
	var lastRequestsMtx sync.Mutex

	var alreadyRequestedUrls = newUnique()
	var uniqueUrls = newUnique()

	urlsChan <- initialURL

	workersAmount := maxRequestsPerSecond * maxRequestsToWorkersMultiplier
	for i := 0; i < workersAmount; i++ {
		go func() {
			for u := range urlsChan {
				glog.V(4).Infof("urlsChan after reading len: %+v", len(urlsChan))
				lastRequestsMtx.Lock()
				if sleep := func() *time.Duration {
					if maxRequestsPerSecond <= 0 {
						return nil
					}
					var obsoleteUpTo = -1
					for i, lr := range lastRequests {
						if lr.Before(time.Now().Add(-limitPeriod)) {
							obsoleteUpTo = i
							continue
						}
						break
					}
					if obsoleteUpTo >= 0 {
						lastRequests = lastRequests[obsoleteUpTo+1:]
					}
					if len(lastRequests) < maxRequestsPerSecond {
						return nil
					}
					sleep := time.Now().Sub(lastRequests[0])
					return &sleep
				}(); sleep != nil {
					urlsChan <- u // we are not processing this url,
					// just returning it to channel and sleeping
					// to avoid site abuse
					glog.V(4).Infof("sleeping for %s, lastRequests: %+v", sleep, timesStr(lastRequests))
					lastRequestsMtx.Unlock()
					time.Sleep(*sleep)
					continue
				}
				lastRequests = append(lastRequests, time.Now())
				lastRequestsMtx.Unlock()

				alreadyRequestedUrls.add(u)
				urls, err := getUrlsOnThePage(u)
				if err != nil {
					glog.Error(err)
					return
				}
				glog.V(4).Infof("from %s got urls %+v ", u, urls)
				for resultUrl := range urls {
					if !strings.HasPrefix(resultUrl, parsedUrl.Scheme) {
						// this for case of a not absolute url
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
					// unescape for proper read of # and query string
					resultUrl, err = url.QueryUnescape(resultUrl)
					if err != nil {
						glog.Error(err)
						continue
					}
					if !uniqueUrls.addIfNotContains(resultUrl) {
						continue
					}
					fmt.Println(resultUrl)
					if alreadyRequestedUrls.contains(resultUrl) {
						glog.V(4).Infof("url %+v already requested, skipping ...", resultUrl)
						continue
					}
					urlsChan <- resultUrl
					glog.V(4).Infof("urlsChan after writing: len: %+v", len(urlsChan))
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

type unique struct {
	mtx  sync.RWMutex
	keys map[string]struct{}
}

func newUnique() *unique {
	u := unique{}
	u.keys = make(map[string]struct{})
	return &u
}

func (u *unique) add(k string) {
	u.mtx.Lock()
	defer u.mtx.Unlock()
	u.keys[k] = struct{}{}
}

func (u *unique) contains(k string) bool {
	u.mtx.RLock()
	defer u.mtx.RUnlock()
	_, ok := u.keys[k]
	return ok
}

func (u *unique) addIfNotContains(k string) (added bool) {
	u.mtx.Lock()
	defer u.mtx.Unlock()
	_, ok := u.keys[k]
	if ok {
		return false
	}
	u.keys[k] = struct{}{}
	return true
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
