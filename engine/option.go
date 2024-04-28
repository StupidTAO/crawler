package engine

import (
	"github.com/StupidTAO/crawler/spider"
	"go.uber.org/zap"
)

type Option func(opts *options)

type options struct {
	WorkCount   int
	Fetcher     spider.Fetcher
	Logger      *zap.Logger
	Seeds       []*spider.Task
	scheduler   Scheduler
	registryURL string
	Storage     spider.Storage
}

var defaultOptions = options{
	Logger: zap.NewNop(),
}

func WithStorage(s spider.Storage) Option {
	return func(opts *options) {
		opts.Storage = s
	}
}

func WithRegistryURL(registryURL string) Option {
	return func(opts *options) {
		opts.registryURL = registryURL
	}
}

func WithLogger(logger *zap.Logger) Option {
	return func(opts *options) {
		opts.Logger = logger
	}
}

func WithFetcher(fetcher spider.Fetcher) Option {
	return func(opts *options) {
		opts.Fetcher = fetcher
	}
}

func WithWorkCount(workCount int) Option {
	return func(opts *options) {
		opts.WorkCount = workCount
	}
}

func WithSeeds(seed []*spider.Task) Option {
	return func(opts *options) {
		opts.Seeds = seed
	}
}

func WithScheduler(scheduler Scheduler) Option {
	return func(opts *options) {
		opts.scheduler = scheduler
	}
}
