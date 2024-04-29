package master

import (
	"context"
	"fmt"
	"github.com/StupidTAO/crawler/cmd/worker"
	"github.com/StupidTAO/crawler/generator"
	"github.com/StupidTAO/crawler/log"
	"github.com/StupidTAO/crawler/master"
	"github.com/StupidTAO/crawler/proto/crawler"
	"github.com/StupidTAO/crawler/spider"
	grpccli "github.com/go-micro/plugins/v4/client/grpc"
	"github.com/go-micro/plugins/v4/config/encoder/toml"
	etcdReg "github.com/go-micro/plugins/v4/registry/etcd"
	gs "github.com/go-micro/plugins/v4/server/grpc"
	"github.com/go-micro/plugins/v4/wrapper/breaker/hystrix"
	ratePlugin "github.com/go-micro/plugins/v4/wrapper/ratelimiter/ratelimit"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/juju/ratelimit"
	"github.com/spf13/cobra"
	"go-micro.dev/v4"
	"go-micro.dev/v4/client"
	"go-micro.dev/v4/config"
	"go-micro.dev/v4/config/reader"
	"go-micro.dev/v4/config/reader/json"
	"go-micro.dev/v4/config/source"
	"go-micro.dev/v4/config/source/file"
	"go-micro.dev/v4/registry"
	"go-micro.dev/v4/server"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"net/http"
	"strconv"
	"time"
)

const (
	RESOURCEPATH = "/resource"

	ADDRESOURCE = iota
	DELETERESOURCE

	ServiceName = "go.micro.server.worker"
)

var MasterCmd = &cobra.Command{
	Use:   "master",
	Short: "run master service.",
	Long:  "run master service.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		Run()
	},
}

func init() {
	MasterCmd.Flags().StringVar(
		&masterID, "id", "", "set master id")
	MasterCmd.Flags().StringVar(
		&HTTPListenAddress, "http", ":8081", "set HTTP listen address")
	MasterCmd.Flags().StringVar(
		&GRPCListenAddress, "grpc", ":9091", "set GRPC listen address")
	MasterCmd.Flags().StringVar(
		&PProfLlistenAddress, "pprof", ":9981", "set GRPC listen address")
	MasterCmd.Flags().StringVar(
		&cfgFile, "config", "config.toml", "set master id")
	MasterCmd.Flags().StringVar(
		&podIP, "podip", "", "set worker id")
}

var masterID string
var HTTPListenAddress string
var GRPCListenAddress string
var PProfLlistenAddress string
var cfgFile string
var podIP string

func Run() {
	//start pprof
	go func() {
		if err := http.ListenAndServe(PProfLlistenAddress, nil); err != nil {
			panic(err)
		}
	}()

	var (
		err    error
		logger *zap.Logger
	)

	// load config
	enc := toml.NewEncoder()
	cfg, err := config.NewConfig(config.WithReader(json.NewReader(reader.WithEncoder(enc))))
	err = cfg.Load(file.NewSource(
		file.WithPath(cfgFile),
		source.WithEncoder(enc),
	))

	if err != nil {
		panic(err)
	}

	// log
	logText := cfg.Get("logLevel").String("INFO")
	logLevel, err := zapcore.ParseLevel(logText)
	if err != nil {
		panic(err)
	}
	plugin := log.NewStdoutPlugin(logLevel)
	logger = log.NewLogger(plugin)
	logger.Info("log init end")

	// set zap global logger
	zap.ReplaceGlobals(logger)

	var sconfig ServerConfig
	if err := cfg.Get("MasterServer").Scan(&sconfig); err != nil {
		logger.Error("get GRPC Server config failed", zap.Error(err))
		return
	}
	logger.Sugar().Debugf("grpc server config,%+v", sconfig)

	reg := etcdReg.NewRegistry(registry.Addrs(sconfig.RegistryAddress))

	//init tasks
	var tcfg []spider.TaskConfig
	if err := cfg.Get("Tasks").Scan(&tcfg); err != nil {
		logger.Error("init seed tasks", zap.Error(err))
		return
	}
	seeds := worker.ParseTaskConfig(logger, nil, nil, tcfg)
	logger.Debug("", zap.Int("len (seed) is : ", len(seeds)))
	m, err := master.New(
		masterID,
		master.WithLogger(logger.Named("master")),
		master.WithGRPCAddress(GRPCListenAddress),
		master.WithRegistryURL(sconfig.RegistryAddress),
		master.WithRegistry(reg),
		master.WithSeeds(seeds),
	)
	if err != nil {
		logger.Error("init master failed", zap.Error(err))
		return
	}

	// start http proxy to GRPC
	go RunHTTPServer(sconfig)

	// start grpc server
	RunGRPCServer(m, logger, reg, sconfig)
}

type ServerConfig struct {
	RegistryAddress  string
	RegisterTTL      int
	RegisterInterval int
	Name             string
	ClientTimeOut    int
}

func RunGRPCServer(MasterService *master.Master, logger *zap.Logger, reg registry.Registry, cfg ServerConfig) {
	b := ratelimit.NewBucketWithRate(0.5, 1)
	if masterID == "" {
		if podIP != "" {
			ip := generator.IDbyIP(podIP)
			masterID = strconv.Itoa(int(ip))
		} else {
			masterID = fmt.Sprintf("%d", time.Now().UnixNano())
		}
	}
	zap.S().Debugf("master id: ", masterID)

	service := micro.NewService(
		micro.Server(gs.NewServer(
			server.Id(masterID),
		)),
		micro.Address(GRPCListenAddress),
		micro.Registry(reg),
		micro.RegisterTTL(time.Duration(cfg.RegisterTTL)*time.Second),
		micro.RegisterInterval(time.Duration(cfg.RegisterInterval)*time.Second),
		micro.WrapHandler(logWrapper(logger)),
		micro.Name(cfg.Name),
		micro.Client(grpccli.NewClient()),
		micro.WrapHandler(ratePlugin.NewHandlerWrapper(b, false)),
		micro.WrapClient(hystrix.NewClientWrapper()),
	)

	hystrix.ConfigureCommand("go.micro.server.master.CrawlerMaster.AddResource", hystrix.CommandConfig{
		Timeout:                10000,
		MaxConcurrentRequests:  100,
		RequestVolumeThreshold: 10,
		SleepWindow:            6000,
		ErrorPercentThreshold:  30,
	})

	cli := crawler.NewCrawlerMasterService(cfg.Name, service.Client())
	MasterService.SetForwardCli(cli)

	// 设置micro 客户端默认超时时间为10秒钟
	if err := service.Client().Init(client.RequestTimeout(time.Duration(cfg.ClientTimeOut) * time.Second)); err != nil {
		logger.Sugar().Error("micro client init error. ", zap.String("error:", err.Error()))
		return
	}

	service.Init()

	if err := crawler.RegisterCrawlerMasterHandler(service.Server(), MasterService); err != nil {
		logger.Fatal("register handler failed", zap.Error(err))
	}

	if err := service.Run(); err != nil {
		logger.Fatal("grpc server stop", zap.Error(err))
	}
}

func RunHTTPServer(cfg ServerConfig) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	defer cancel()

	mux := runtime.NewServeMux()
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	if err := crawler.RegisterCrawlerMasterGwFromEndpoint(ctx, mux, GRPCListenAddress, opts); err != nil {
		zap.L().Fatal("Register backend grpc server endpoint failed")
	}
	zap.S().Debugf("start http server listening on %v proxy to grpc server;%v", HTTPListenAddress, GRPCListenAddress)
	if err := http.ListenAndServe(HTTPListenAddress, mux); err != nil {
		zap.L().Fatal("http listenAndServe failed", zap.Error(err))
	}
}

func logWrapper(log *zap.Logger) server.HandlerWrapper {
	return func(fn server.HandlerFunc) server.HandlerFunc {
		return func(ctx context.Context, req server.Request, rsp interface{}) error {

			log.Info("receive request",
				zap.String("method", req.Method()),
				zap.String("Service", req.Service()),
				zap.Reflect("request param:", req.Body()),
				zap.String("Endpoint", req.Endpoint()),
			)

			err := fn(ctx, req, rsp)

			return err
		}
	}
}
