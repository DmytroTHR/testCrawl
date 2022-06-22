package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/gocolly/colly/v2"
)

type ExecutionResult struct {
	StatusCode int
	Status     string
	IsError    bool
}

func (app *Config) WriteResultToFile(execResult *sync.Map) {
	file, err := os.Create(resultFile)
	if err != nil {
		log.Panic(err)
	}
	defer file.Close()

	execResult.Range(func(link, value interface{}) bool {
		curResult := value.(ExecutionResult)
		strResult := fmt.Sprintf("Err:%t, %d - %s :\t%s\n",
			curResult.IsError, curResult.StatusCode, curResult.Status, link)
		file.WriteString(strResult)
		return true
	})

	log.Println("Result saved to file: ", resultFile)
}

func (app *Config) DoCrawl(ctx context.Context) *sync.Map {
	res := sync.Map{}

	col := prepareCrawler(app.URL)

	col.OnRequest(func(request *colly.Request) {
		abortRequestOnContext(ctx, request)
	})
	col.OnResponseHeaders(func(response *colly.Response) {
		request := response.Request
		abortRequestOnContext(ctx, request)
	})

	col.OnHTML(`[href]`, searchForLinks(&res, "href"))
	col.OnHTML(`[src]`, searchForLinks(&res, "src"))

	col.OnScraped(func(r *colly.Response) {
		res.Store(r.Request.URL.String(), ExecutionResult{
			StatusCode: r.StatusCode,
			Status:     http.StatusText(r.StatusCode),
			IsError:    r.StatusCode >= 203,
		})
	})

	endpointPath := app.URL.String()
	res.Store(endpointPath, ExecutionResult{})
	col.Visit(endpointPath)

	col.Wait()

	removeEmptyResults(&res)
	return &res
}

func removeEmptyResults(results *sync.Map) {
	results.Range(func(link, value interface{}) bool {
		curResult := value.(ExecutionResult)
		if (curResult == ExecutionResult{}) {
			results.Delete(link)
		}
		return true
	})
}

func abortRequestOnContext(ctx context.Context, request *colly.Request) {
	select {
	case <-ctx.Done():
		request.Abort()
	default:
	}
}

func prepareCrawler(urlToCrawl *url.URL) *colly.Collector {
	regexForHost := fmt.Sprintf(".*%s", strings.ReplaceAll(urlToCrawl.Hostname(), ".", "\\."))
	regexForEndpoint := fmt.Sprintf("%s.*", strings.ReplaceAll(urlToCrawl.String(), ".", "\\."))

	col := colly.NewCollector(
		colly.Async(),
		colly.MaxDepth(crawlingDepth),
		colly.IgnoreRobotsTxt(),
		colly.ParseHTTPErrorResponse(),
		colly.URLFilters(
			regexp.MustCompile(regexForEndpoint),
		),
	)

	col.Limit(&colly.LimitRule{
		Parallelism:  runtime.NumCPU() / 2,
		RandomDelay:  defaultTimeout / 10,
		DomainRegexp: regexForHost,
	})
	col.SetRequestTimeout(defaultTimeout)

	return col
}

func searchForLinks(res *sync.Map, attrName string) func(e *colly.HTMLElement) {
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
