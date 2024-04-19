package main

import (
	"context"
	"fmt"
	"github.com/StupidTAO/crawler/collect"
	"github.com/StupidTAO/crawler/collector"
	"github.com/StupidTAO/crawler/collector/sqlstorage"
	"github.com/StupidTAO/crawler/engine"
	"github.com/StupidTAO/crawler/limiter"
	"github.com/StupidTAO/crawler/log"
	pb "github.com/StupidTAO/crawler/proto/greeter"
	"github.com/StupidTAO/crawler/proxy"
	etcdReg "github.com/go-micro/plugins/v4/registry/etcd"
	gs "github.com/go-micro/plugins/v4/server/grpc"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go-micro.dev/v4"
	"go-micro.dev/v4/registry"
	"go-micro.dev/v4/server"
	"go.uber.org/zap/zapcore"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"net/http"
	"time"
)

func main() {

	// log
	plugin := log.NewStdoutPlugin(zapcore.InfoLevel)
	logger := log.NewLogger(plugin)
	logger.Info("log init end")

	// proxy
	proxyURLs := []string{"http://127.0.0.1:7890", "http://127.0.0.1:7890"}
	p, err := proxy.RoundRobinProxySwitcher(proxyURLs...)
	if err != nil {
		logger.Error("RoundRobinProxySwitcher failed")
	}

	//fetcher
	var f collect.Fetcher = &collect.BrowserFetch{
		Timeout: 3000 * time.Second,
		Logger:  logger,
		Proxy:   p,
	}

	//storage
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

	//speed limiter
	secondLimit := rate.NewLimiter(limiter.Per(1, 2*time.Second), 1)
	minuteLimit := rate.NewLimiter(limiter.Per(20, 1*time.Minute), 20)
	multiLimiter := limiter.MultiLimiter(secondLimit, minuteLimit)

	seeds := make([]*collect.Task, 0, 1024)
	seeds = append(seeds, &collect.Task{
		Property: collect.Property{
			Name: "douban_book_list",
			//Name: "js_find_douban_sun_room",
			//Name: "find_douban_sun_room",
		},
		Fetcher: f,
		Storage: storage,
		Limit:   multiLimiter,
	})

	s := engine.NewEngine(
		engine.WithFetcher(f),
		engine.WithLogger(logger),
		engine.WithWorkCount(10),
		engine.WithSeeds(seeds),
		engine.WithScheduler(engine.NewSchedule()),
	)

	//worker start
	s.Run()

	//start http proxy to GRPC
	go HandleHTTP()

	//start grpc server
	reg := etcdReg.NewRegistry(
		registry.Addrs(":2379"),
	)

	service := micro.NewService(
		micro.Server(gs.NewServer(
			server.Id("1"),
		)),
		micro.Address(":9090"),
		micro.Registry(reg),
		micro.Name("go.micro.server.worker"),
	)
	service.Init()
	pb.RegisterGreeterHandler(service.Server(), new(Greeter))
	if err := service.Run(); err != nil {
		logger.Fatal("grpc server stop!")
		return
	}
}

type Greeter struct{}

func (g *Greeter) Hello(ctx context.Context, req *pb.Request, resp *pb.Response) error {
	resp.Greeting = "Hello " + req.Name
	return nil
}

func HandleHTTP() {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	mux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithInsecure()}

	err := pb.RegisterGreeterGwFromEndpoint(ctx, mux, "localhost:9090", opts)
	if err != nil {
		fmt.Println(err)
		return
	}

	http.ListenAndServe(":8080", mux)
}
