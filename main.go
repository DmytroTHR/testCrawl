package main

import (
	"context"
	"fmt"
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
	defaultTimeout   = 5 * time.Second
	crawlingDepth    = 3
	providedLink     = "https://4club.com.ua/"
	wholeTestTimeout = time.Minute
	resultFile       = "results.txt"
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
	log.Printf("Checking path: %s", providedLink)

	providedURL, err := url.Parse(providedLink)
	if err != nil {
		log.Panic(err)
	}

	app := &Config{
		URL: providedURL,
	}

	testResult := []TestResult{}

	ctx, cancel := context.WithTimeout(context.Background(), wholeTestTimeout)
	defer cancel()
	execResult := app.LinksCheck(ctx)
	execResult.Range(func(link, value interface{}) bool {
		valueRes := value.(ExecutionResult)
		if valueRes == (ExecutionResult{}) {
			return true
		}
		testResult = append(testResult, TestResult{
			ExecutionResult: valueRes,
			Link:            link.(string),
		})
		return true
	})

	sort.Slice(testResult, func(i, j int) bool {
		return testResult[i].StatusCode > testResult[j].StatusCode
	})

	app.WriteResultToFile(testResult)
}

func (app *Config) WriteResultToFile(testResult []TestResult) {
	file, err := os.Create(resultFile)
	if err != nil {
		log.Panic(err)
	}
	defer file.Close()

	for _, test := range testResult {
		file.WriteString(fmt.Sprintf("Err:%t, %d - %s :\t%s\n", test.IsError, test.StatusCode, test.Status, test.Link))
	}
	log.Println("Result saved to file: ", resultFile)
}

func (app *Config) LinksCheck(ctx context.Context) *sync.Map {
	res := sync.Map{}
	hostname := app.URL.Hostname()

	col := colly.NewCollector(
		colly.AllowedDomains(hostname),
		colly.MaxDepth(crawlingDepth),
		colly.MaxBodySize(1024*1024),
		colly.IgnoreRobotsTxt(),
		colly.Async(),
		colly.ParseHTTPErrorResponse(),
		colly.UserAgent("Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:101.0) Gecko/20100101 Firefox/101.0"),
	)

	col.Limit(&colly.LimitRule{
		Parallelism:  runtime.NumCPU() / 2,
		RandomDelay:  defaultTimeout / 100,
		DomainRegexp: fmt.Sprintf(".*%s.*", strings.ReplaceAll(hostname, ".", "\\.")),
	})
	col.SetRequestTimeout(defaultTimeout)

	col.OnRequest(func(request *colly.Request) {
		select {
		case <-ctx.Done():
			request.Abort()
		default:
		}
	})
	col.OnResponseHeaders(func(response *colly.Response) {
		request := response.Request
		select {
		case <-ctx.Done():
			request.Abort()
		default:
		}
	})

	col.OnHTML(`[href]`, collyFunc(&res, ctx, "href"))
	col.OnHTML(`[src]`, collyFunc(&res, ctx, "src"))

	col.OnScraped(func(r *colly.Response) {
		res.Store(r.Request.URL.String(), ExecutionResult{
			StatusCode: r.StatusCode,
			Status:     http.StatusText(r.StatusCode),
			IsError:    r.StatusCode >= 203,
		})
	})

	mainLink := app.URL.String()
	res.Store(mainLink, ExecutionResult{})
	col.Visit(mainLink)
	col.Wait()

	log.Println(col)

	return &res
}

func collyFunc(res *sync.Map, ctx context.Context, attrName string) func(e *colly.HTMLElement) {
	return func(e *colly.HTMLElement) {
		link := e.Request.AbsoluteURL(e.Attr(attrName))
		if link != "" {
			if _, ok := res.Load(link); !ok {
				res.Store(link, ExecutionResult{})
				e.Request.Visit(link)
			}
		}
	}
}
