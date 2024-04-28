package engine

import (
	"context"
	"fmt"
	"github.com/StupidTAO/crawler/master"
	"github.com/StupidTAO/crawler/parse/doubanbook"
	"github.com/StupidTAO/crawler/parse/doubangroup"
	"github.com/StupidTAO/crawler/parse/doubangroupjs"
	"github.com/StupidTAO/crawler/spider"
	"github.com/robertkrimen/otto"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
	"runtime/debug"
	"strings"
	"sync"
)

func init() {
	Store.Add(doubangroup.DoubanGroupTask)
	Store.Add(doubanbook.DoubanBookTask)
	Store.AddJSTask(doubangroupjs.DoubanGroupJSTask)
}

func (c *CrawlerStore) Add(task *spider.Task) {
	c.hash[task.Name] = task
	c.list = append(c.list, task)
}

// 用于动态规则添加请求。
func AddJsReqs(jres []map[string]interface{}) []*spider.Request {
	reqs := make([]*spider.Request, 0)

	for _, jreq := range jres {
		req := &spider.Request{}
		u, ok := jreq["Url"].(string)
		if !ok {
			return nil
		}
		req.URL = u
		req.RuleName, _ = jreq["RuleName"].(string)
		req.Method, _ = jreq["Method"].(string)
		req.Priority, _ = jreq["Priority"].(int64)
		reqs = append(reqs, req)
	}

	return reqs
}

// 用于动态规则添加请求。
func AddJsReq(jreq map[string]interface{}) []*spider.Request {
	reqs := make([]*spider.Request, 0)
	req := &spider.Request{}
	u, ok := jreq["Url"].(string)
	if !ok {
		return nil
	}
	req.URL = u
	req.RuleName, _ = jreq["RuleName"].(string)
	req.Method, _ = jreq["Method"].(string)
	req.Priority, _ = jreq["Priority"].(int64)
	reqs = append(reqs, req)
	return reqs
}

func (c *CrawlerStore) AddJSTask(m *spider.TaskModle) {
	task := &spider.Task{
		//Property: m.Property,
	}

	task.Rule.Root = func() ([]*spider.Request, error) {
		vm := otto.New()
		vm.Set("AddJsReq", AddJsReqs)
		v, err := vm.Eval(m.Root)
		if err != nil {
			return nil, err
		}

		e, err := v.Export()
		if err != nil {
			return nil, err
		}
		return e.([]*spider.Request), nil
	}

	for _, r := range m.Rules {
		parseFunc := func(parse string) func(ctx *spider.Context) (spider.ParseResult, error) {
			return func(ctx *spider.Context) (spider.ParseResult, error) {
				vm := otto.New()
				vm.Set("ctx", ctx)
				v, err := vm.Eval(parse)
				if err != nil {
					return spider.ParseResult{}, nil
				}
				e, err := v.Export()
				if err != nil {
					return spider.ParseResult{}, nil
				}
				if e == nil {
					return spider.ParseResult{}, nil
				}
				return e.(spider.ParseResult), err
			}
		}(r.ParseFunc)
		if task.Rule.Trunk == nil {
			task.Rule.Trunk = make(map[string]*spider.Rule, 0)
		}
		task.Rule.Trunk[r.Name] = &spider.Rule{
			ParseFunc: parseFunc,
		}
	}
	if task.Name == "" {
		fmt.Println("AddJSTask() task.Name = ", task.Name)
		return
	}
	c.hash[task.Name] = task
	c.list = append(c.list, task)
}

// 全局蜘蛛种类实例
var Store = &CrawlerStore{
	list: []*spider.Task{},
	hash: map[string]*spider.Task{},
}

func GetFields(taskName string, ruleName string) []string {
	return Store.hash[taskName].Rule.Trunk[ruleName].ItemFields
}

type CrawlerStore struct {
	list []*spider.Task
	hash map[string]*spider.Task
}

type Crawler struct {
	id  string
	out chan spider.ParseResult

	Visited     map[string]bool
	VisitedLock sync.Mutex

	failures    map[string]*spider.Request //失败请求id -> 失败请求
	failureLock sync.Mutex

	resources map[string]*master.ResourceSpec
	rlock     sync.Mutex

	etcdCli *clientv3.Client
	options
}

type Scheduler interface {
	Schedule()
	Push(...*spider.Request)
	Pull() *spider.Request
}

type Schedule struct {
	requestCh   chan *spider.Request
	workerCh    chan *spider.Request
	priReqQueue []*spider.Request
	reqQueue    []*spider.Request
	Logger      *zap.Logger
}

// 函数式选项模式
func NewEngine(opts ...Option) (*Crawler, error) {
	options := defaultOptions
	for _, opt := range opts {
		opt(&options)
	}
	e := &Crawler{}
	e.Visited = make(map[string]bool, 128)
	e.out = make(chan spider.ParseResult)
	e.failures = make(map[string]*spider.Request)
	e.options = options

	//任务加上默认的采集器与存储器
	for _, task := range Store.list {
		task.Fetcher = e.Fetcher
		task.Storage = e.Storage
	}

	endpoints := []string{e.registryURL}
	cli, err := clientv3.New(clientv3.Config{Endpoints: endpoints})
	if err != nil {
		return nil, err
	}
	e.etcdCli = cli

	return e, nil
}

func NewSchedule() *Schedule {
	s := &Schedule{}
	requestCh := make(chan *spider.Request)
	workerCh := make(chan *spider.Request)
	s.requestCh = requestCh
	s.workerCh = workerCh
	return s
}

func (e *Crawler) Run(id string, cluster bool) {
	e.id = id
	if !cluster {
		e.handleSeeds()
	}

	go e.loadResource()
	go e.watchResource()
	go e.Schedule()
	for i := 0; i < e.WorkCount; i++ {
		go e.CreateWork()
	}
	e.HandleResult()
}

func (s *Schedule) Push(reqs ...*spider.Request) {
	for _, req := range reqs {
		s.requestCh <- req
	}
}

func (s *Schedule) Pull() *spider.Request {
	r := <-s.workerCh
	return r
}

func (s *Schedule) Output() *spider.Request {
	r := <-s.workerCh
	return r
}

func (s *Schedule) Schedule() {
	var req *spider.Request
	var ch chan *spider.Request

	for {
		if req == nil && len(s.priReqQueue) > 0 {
			req = s.priReqQueue[0]
			s.priReqQueue = s.priReqQueue[1:]
			ch = s.workerCh
		}
		if req == nil && len(s.reqQueue) > 0 {
			req = s.reqQueue[0]
			s.reqQueue = s.reqQueue[1:]
			ch = s.workerCh
		}

		// 请求校验
		if req != nil {
			if err := req.Check(); err != nil {
				zap.L().Debug("check failed",
					zap.Error(err),
				)
				req = nil
				ch = nil
				continue
			}
		}

		select {
		case r := <-s.requestCh:
			if r.Priority > 0 {
				s.priReqQueue = append(s.priReqQueue, r)
			} else {
				s.reqQueue = append(s.reqQueue, r)
			}
			//ch为nil时，向ch写数据将阻塞
		case ch <- req:
			req = nil
			ch = nil
		}
	}
}

func (e *Crawler) Schedule() {
	e.scheduler.Schedule()
}

func (e *Crawler) handleSeeds() {
	var reqs []*spider.Request
	for _, task := range e.Seeds {
		t, ok := Store.hash[task.Name]
		if !ok {
			e.Logger.Error("can not find preset tasks", zap.String("task name", task.Name))
			continue
		}

		task.Rule = t.Rule

		rootreqs, err := task.Rule.Root()
		if err != nil {
			e.Logger.Error("get root failed", zap.Error(err))
			continue
		}

		for _, req := range rootreqs {
			req.Task = task
		}

		reqs = append(reqs, rootreqs...)
	}
	fmt.Println("handleSeeds() len(reqs) = ", len(reqs))
	go e.scheduler.Push(reqs...)
}

func (e *Crawler) CreateWork() {
	defer func() {
		if err := recover(); err != nil {
			e.Logger.Error("worker panic",
				zap.Any("err", err),
				zap.String("stack", string(debug.Stack())))
		}
	}()

	for {
		req := e.scheduler.Pull()

		if err := req.Check(); err != nil {
			e.Logger.Error("check failed", zap.Error(err))
			continue
		}

		if !req.Task.Reload && e.HasVisited(req) {
			e.Logger.Debug("request has visited", zap.String("url:", req.URL))
			continue
		}

		e.StoreVisited(req)
		fmt.Println("CreateWork() req.Task.Name = ", req.Task.Name)
		fmt.Println("CreateWork() req = ", req)

		body, err := req.Fetch()
		if err != nil {
			e.Logger.Error("can't fetch ", zap.Error(err), zap.String("url", req.URL))
			e.SetFailure(req)
			continue
		}

		rule := req.Task.Rule.Trunk[req.RuleName]
		ctx := &spider.Context{
			Body: body,
			Req:  req,
		}
		result, err := rule.ParseFunc(ctx)
		if err != nil {
			e.Logger.Error("ParseFunc failed ", zap.Error(err), zap.String("url", req.URL))
			continue
		}

		if len(result.Requests) > 0 {
			go e.scheduler.Push(result.Requests...)
		}

		e.out <- result
	}
}

func (e *Crawler) HandleResult() {
	for {
		select {
		case result := <-e.out:
			for _, item := range result.Items {
				switch d := item.(type) {
				case *spider.DataCell:
					err := d.Task.Storage.Save(d)
					if err != nil {
						e.Logger.Error("HandleResult() task store sql error!", zap.Error(err))
						continue
					}
					e.Logger.Sugar().Info("get result: ", item)
				}
			}
		}
	}
}

func (e *Crawler) HasVisited(r *spider.Request) bool {
	e.VisitedLock.Lock()
	defer e.VisitedLock.Unlock()
	unique := r.Unique()
	return e.Visited[unique]
}

func (e *Crawler) StoreVisited(reqs ...*spider.Request) {
	e.VisitedLock.Lock()
	defer e.VisitedLock.Unlock()

	for _, r := range reqs {
		unique := r.Unique()
		e.Visited[unique] = true
	}
}

func (e *Crawler) SetFailure(req *spider.Request) {
	//如果网站不可以重复爬取，将访问记录删除
	if !req.Task.Reload {
		e.VisitedLock.Lock()
		unique := req.Unique()
		delete(e.Visited, unique)
		e.VisitedLock.Unlock()
	}
	e.failureLock.Lock()
	defer e.failureLock.Unlock()

	//读取失败任务，并推送至任务处理通道
	if _, ok := e.failures[req.Unique()]; !ok {
		// 首次失败时，再重新执行一次
		e.failures[req.Unique()] = req
		e.scheduler.Push(req)
		e.Logger.Debug("SetFailure() set failure req once!")
	}
	//todo: 失败两次，加载到失败队列中
}

func (c *Crawler) watchResource() {
	watch := c.etcdCli.Watch(context.Background(), master.RESOURCEPATH, clientv3.WithPrefix(), clientv3.WithPrevKV())
	for w := range watch {
		if w.Err() != nil {
			c.Logger.Error("watch resource failed", zap.Error(w.Err()))
			continue
		}
		if w.Canceled {
			c.Logger.Error("watch resource canceled")
			return
		}
		for _, ev := range w.Events {
			switch ev.Type {
			case clientv3.EventTypePut:
				spec, err := master.Decode(ev.Kv.Value)
				if err != nil || spec == nil {
					c.Logger.Error("decode etcd value failed", zap.Error(err))
					continue
				}
				if ev.IsCreate() {
					c.Logger.Info("receive create resource", zap.Any("spec", spec))

				} else if ev.IsModify() {
					c.Logger.Info("receive update resource", zap.Any("spec", spec))
				}

				c.rlock.Lock()
				c.runTasks(spec.Name)
				c.rlock.Unlock()

			case clientv3.EventTypeDelete:
				spec, err := master.Decode(ev.PrevKv.Value)
				c.Logger.Info("receive delete resource", zap.Any("spec", spec))
				if err != nil {
					c.Logger.Error("decode etcd value failed", zap.Error(err))
				}

				c.rlock.Lock()
				c.deleteTasks(spec.Name)
				c.rlock.Unlock()
			}
		}
	}
}

func getID(assignedNode string) string {
	s := strings.Split(assignedNode, "|")
	if len(s) < 2 {
		return ""
	}
	return s[0]
}

func (c *Crawler) loadResource() error {
	resp, err := c.etcdCli.Get(context.Background(), master.RESOURCEPATH, clientv3.WithPrefix(), clientv3.WithSerializable())
	if err != nil {
		return fmt.Errorf("etcd get failed")
	}

	resources := make(map[string]*master.ResourceSpec)
	for _, kv := range resp.Kvs {
		r, err := master.Decode(kv.Value)
		if err == nil && r != nil {
			id := getID(r.AssignedNode)
			if len(id) > 0 && c.id == id {
				resources[r.Name] = r
			}
		}
	}
	c.Logger.Info("leader init load resource", zap.Int("lenth", len(resources)))
	c.Logger.Info("leader init load resource", zap.Any("resources", resources))
	c.rlock.Lock()
	defer c.rlock.Unlock()
	c.resources = resources
	for _, r := range resources {
		c.runTasks(r.Name)
	}

	return nil
}

func (c *Crawler) deleteTasks(taskName string) {
	t, ok := Store.hash[taskName]
	if !ok {
		c.Logger.Error("can not find preset tasks", zap.String("task name", taskName))
		return
	}
	t.Closed = true
	delete(c.resources, taskName)
}

func (c *Crawler) runTasks(taskName string) {
	if len(taskName) == 0 {
		c.Logger.Error("can not find preset tasks", zap.Int("len(taskName)", len(taskName)))
		return
	}

	if _, ok := c.resources[taskName]; ok {
		c.Logger.Info("task has runing", zap.String("name", taskName))
		return
	}

	t, ok := Store.hash[taskName]
	if !ok {
		c.Logger.Error("can not find preset tasks", zap.String("task name", taskName))
		return
	}
	t.Closed = false
	res, err := t.Rule.Root()

	if err != nil {
		c.Logger.Error("get root failed",
			zap.Error(err),
		)
		return
	}

	for _, req := range res {
		req.Task = t
	}
	c.scheduler.Push(res...)
}
