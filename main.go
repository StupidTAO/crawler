package main

import (
	"github.com/StupidTAO/crawler/collect"
	"github.com/StupidTAO/crawler/collector"
	"github.com/StupidTAO/crawler/collector/sqlstorage"
	"github.com/StupidTAO/crawler/engine"
	"github.com/StupidTAO/crawler/log"
	"github.com/StupidTAO/crawler/proxy"
	"go.uber.org/zap/zapcore"
	"time"
)

func main() {

	// log
	plugin := log.NewStdoutPlugin(zapcore.DebugLevel)
	logger := log.NewLogger(plugin)
	logger.Info("log init end")

	// proxy
	proxyURLs := []string{"http://127.0.0.1:7890", "http://127.0.0.1:7890"}
	p, err := proxy.RoundRobinProxySwitcher(proxyURLs...)
	if err != nil {
		logger.Error("RoundRobinProxySwitcher failed")
	}

	var f collect.Fetcher = &collect.BrowserFetch{
		Timeout: 3000 * time.Second,
		Logger:  logger,
		Proxy:   p,
	}

	var storage collector.Storage
	storage, err = sqlstorage.New(
		sqlstorage.WithSqlUrl("root:123456@tcp(127.0.0.1:3326)/crawler?charset=utf8"),
		sqlstorage.WithLogger(logger.Named("sqlDB")),
		sqlstorage.WithBatchCount(1),
	)
	if err != nil {
		logger.Error("create sqlstorage failed")
		return
	}

	seeds := make([]*collect.Task, 0, 1024)
	seeds = append(seeds, &collect.Task{
		Property: collect.Property{
			Name: "douban_book_list",
			//Name: "js_find_douban_sun_room",
			//Name: "find_douban_sun_room",
		},
		Fetcher: f,
		Storage: storage,
	})

	s := engine.NewEngine(
		engine.WithFetcher(f),
		engine.WithLogger(logger),
		engine.WithWorkCount(10),
		engine.WithSeeds(seeds),
		engine.WithScheduler(engine.NewSchedule()),
	)

	s.Run()
}
