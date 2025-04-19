package main

import (
	"fmt"
	"log"
	"time"

	"github.com/firehourse/cdpkit/browser"
	"github.com/firehourse/cdpkit/config"
	"github.com/firehourse/cdpkit/tab"
)

func main() {
	cfg := config.Config{
		WebSocketURL: "ws://localhost:9222/devtools/browser/<YOUR_ID>",
		TabLimit:     20,
		Timeout:      30 * time.Second,
	}
	bm, err := browser.NewManagerFromConfig(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer bm.Shutdown()

	ctx, cancel, err := bm.NewPageContext()
	if err != nil {
		log.Fatal(err)
	}
	tab := tab.New(ctx, cancel, cfg.Timeout)
	defer tab.Close(bm)

	if err := tab.Navigate("https://example.org", 0); err != nil {
		log.Fatal(err)
	}
	if err := tab.Spoof(); err != nil {
		log.Fatal(err)
	}
	html, err := tab.HTML(0)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(html)
}
