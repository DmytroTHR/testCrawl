package main

import (
	"context"
	"log"
	"net/url"
	"time"
)

const (
	wholeTestTimeout = time.Minute
	defaultTimeout   = 3 * time.Second
	crawlingDepth    = 3
	providedLink     = "https://4club.com.ua/light/"
	resultFile       = "results.txt"
)

type Config struct {
	URL *url.URL
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

	ctx, cancel := context.WithTimeout(context.Background(), wholeTestTimeout)
	defer cancel()

	execResult := app.DoCrawl(ctx)
	app.WriteResultToFile(execResult)
}
