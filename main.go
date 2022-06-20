package main

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
)

const (
	defaultTimeout = 5 * time.Second
)

type Config struct {
	URL *url.URL
}

type ExecutionResult struct {
	StatusCode int
	Status     string
	IsError    bool
}
type TestResult struct {
	ExecutionResult
	Link string
}

func main() {
	if len(os.Args) < 2 {
		log.Panic("You should provide a hostname")
	}
	providedLink := os.Args[1]
	providedURL, err := url.Parse(providedLink)
	if err != nil {
		log.Panic(err)
	}

	app := &Config{
		URL: providedURL,
	}
	log.Printf("Hostname: %s", app.URL.Hostname())
	
	testResult := []TestResult{}

	execResult := app.LinksCheck()
	execResult.Range(func(link, value interface{}) bool {
		valueRes := value.(ExecutionResult)
		testResult = append(testResult, TestResult{
			ExecutionResult: valueRes,
			Link:            link.(string),
		})
		return true
	})

	sort.Slice(testResult, func(i, j int) bool {
		return testResult[i].StatusCode > testResult[j].StatusCode
	})
	for _, test := range testResult {
		log.Printf("Err:%t, %d - %s :\t%s\n", test.IsError, test.StatusCode, test.Status, test.Link)
	}
}

func (app *Config) LinksCheck() *sync.Map {
	res := sync.Map{}

	col := colly.NewCollector(
		colly.AllowedDomains(app.URL.Hostname()),
		colly.MaxDepth(3),
		colly.MaxBodySize(0),
		colly.IgnoreRobotsTxt(),
		colly.Async(),
	)

	col.Limit(&colly.LimitRule{Parallelism: runtime.NumCPU()})
	col.SetRequestTimeout(defaultTimeout)

	col.OnHTML(`[href]`, collyFunc(&res, "href", app.URL.Hostname()))
	col.OnHTML(`[src]`, collyFunc(&res, "href", app.URL.Hostname()))

	col.OnResponse(func(r *colly.Response) {
		res.Store(r.Request.URL.String(), ExecutionResult{
			StatusCode: r.StatusCode,
			Status:     http.StatusText(r.StatusCode),
		})
	})
	col.OnError(func(r *colly.Response, err error) {
		res.Store(r.Request.URL.String(), ExecutionResult{
			StatusCode: r.StatusCode,
			Status:     http.StatusText(r.StatusCode),
			IsError:    true,
		})
	})

	mainLink := app.URL.String()
	res.Store(mainLink, ExecutionResult{})
	col.Visit(mainLink)
	col.Wait()

	log.Println(col)

	return &res
}

func collyFunc(res *sync.Map, attrName, hostname string) func(e *colly.HTMLElement) {
	return func(e *colly.HTMLElement) {
		link := e.Request.AbsoluteURL(e.Attr(attrName))
		if link != "" && strings.Contains(link, hostname) {
			if _, ok := res.Load(link); !ok {
				res.Store(link, ExecutionResult{})
				e.Request.Visit(link)
			}
		}
	}
}
