package engine

import (
	"github.com/StupidTAO/crawler/collect"
	"go.uber.org/zap"
)

type Schedule struct {
	requestCh chan *collect.Request
	workerCh  chan *collect.Request
	out       chan collect.ParseResult
	options
}

// 函数式选项模式
func NewSchedule(opts ...Option) *Schedule {
	options := defaultOptions
	for _, opt := range opts {
		opt(&options)
	}
	s := &Schedule{}
	s.options = options
	return s
}

func (s *Schedule) Run() {
	requestCh := make(chan *collect.Request)
	workerCh := make(chan *collect.Request)
	out := make(chan collect.ParseResult)
	s.requestCh = requestCh
	s.workerCh = workerCh
	s.out = out
	go s.Schedule()
	for i := 0; i < s.WorkCount; i++ {
		go s.CreateWork()
	}
	s.HandleResult()
}

func (s *Schedule) Schedule() {
	//如果使用协程并发调用Schedule()，那么s.Seeds应该加锁
	//该函数是任务的读取与分发，只会由一个协程执行，不会被并发调用
	var reqQueue = s.Seeds
	go func() {
		for {
			var req *collect.Request
			var ch chan *collect.Request

			if len(reqQueue) > 0 {
				req = reqQueue[0]
				reqQueue = reqQueue[1:]
				//todo: 把ch放在闭包外定义，结果就出错了
				ch = s.workerCh
			}
			//case同时满足时，会随机的选择一个case
			select {
			case r := <-s.requestCh:
				reqQueue = append(reqQueue, r)
			case ch <- req:
			}
		}
	}()
}

func (s *Schedule) CreateWork() {
	for {
		r := <-s.workerCh
		body, err := s.Fetcher.Get(r)
		if len(body) < 6000 {
			s.Logger.Error("can't fetch， content less than 6000 byte", zap.Error(err), zap.String("url", r.Url))
			continue
		}

		if err != nil {
			s.Logger.Error("can't fetch", zap.Error(err), zap.String("url", r.Url))
			continue
		}
		result := r.ParseFunc(body, r)
		s.out <- result
	}
}

func (s *Schedule) HandleResult() {
	for {
		select {
		case result := <-s.out:
			//这块实现了广度优先遍历
			for _, req := range result.Requests {
				s.requestCh <- req
			}
			for _, item := range result.Items {
				//todo: sotre
				s.Logger.Sugar().Info("get result: ", item)
			}

		}
	}
}
